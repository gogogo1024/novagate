package novagate

import (
	"context"
	"errors"
	"log"
	"net"
	"syscall"
	"time"

	"github.com/gogogo1024/novagate/protocol"
)

// SetupFunc is an injection point for registering command tables and handlers.
// It is called once before serving starts.
type SetupFunc func(r *Router) error

var ErrNoSetup = errors.New("novagate: setup is required")

type serveOptions struct {
	addr         string
	idleTimeout  time.Duration
	writeTimeout time.Duration
}

type ServeOption func(*serveOptions)

func defaultServeOptions() serveOptions {
	return serveOptions{addr: ":9000", idleTimeout: 5 * time.Minute, writeTimeout: 10 * time.Second}
}

func applyServeOptions(opts []ServeOption) serveOptions {
	so := defaultServeOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&so)
		}
	}
	return so
}

// normalizeAddr resolves addr with options.
//
// Rules:
// - If addr is non-empty, it wins (backward compatible).
// - Otherwise, use WithAddr if provided.
// - Otherwise, use the default addr from defaultServeOptions.
func normalizeAddr(addr string, opts []ServeOption) string {
	if addr != "" {
		return addr
	}
	so := applyServeOptions(opts)
	if so.addr != "" {
		return so.addr
	}
	return ":9000"
}

// WithIdleTimeout configures a per-connection idle timeout.
//
// When set, the server will close the connection after it stays idle for the
// specified duration. Use 0 to disable the idle timeout.
func WithIdleTimeout(d time.Duration) ServeOption {
	return func(o *serveOptions) {
		o.idleTimeout = d
	}
}

// WithWriteTimeout configures a per-write timeout for responses.
//
// When set, each conn.Write for protocol responses will be bounded by the
// specified duration. Use 0 to disable the write timeout.
func WithWriteTimeout(d time.Duration) ServeOption {
	return func(o *serveOptions) {
		o.writeTimeout = d
	}
}

// ListenAndServe starts a TCP listener on addr and serves the Novagate protocol.
// The caller must provide setup to register command mappings and handlers.
func ListenAndServe(addr string, setup SetupFunc) error {
	listener, err := net.Listen("tcp", normalizeAddr(addr, nil))
	if err != nil {
		return err
	}
	return Serve(listener, setup)
}

// WithAddr configures the TCP listen address for ListenAndServe variants.
//
// This option only applies to functions that create a listener internally.
// It has no effect when you call Serve/ServeWithContext with an existing listener.
func WithAddr(addr string) ServeOption {
	return func(o *serveOptions) {
		o.addr = addr
	}
}

// ListenAndServeWithOptions is like ListenAndServe but allows configuring server behavior.

func ListenAndServeWithOptions(addr string, setup SetupFunc, opts ...ServeOption) error {
	return ListenAndServeWithContext(context.Background(), addr, setup, opts...)
}

// ListenAndServeWithOptionsOptions is like ListenAndServeWithOptions but takes the addr from options.
//
// If you don't specify WithAddr, it defaults to ":9000".
func ListenAndServeWithOptionsOptions(setup SetupFunc, opts ...ServeOption) error {
	return ListenAndServeWithContextOptions(context.Background(), setup, opts...)
}

// ListenAndServeWithContext is like ListenAndServeWithOptions but can be stopped via ctx cancellation.
//
// When ctx is canceled, the listener will be closed and Serve will return.
func ListenAndServeWithContext(ctx context.Context, addr string, setup SetupFunc, opts ...ServeOption) error {
	listener, err := net.Listen("tcp", normalizeAddr(addr, opts))
	if err != nil {
		return err
	}
	defer listener.Close()
	return ServeWithContext(ctx, listener, setup, opts...)
}

// ListenAndServeWithContextOptions is like ListenAndServeWithContext but takes the addr from options.
//
// If you don't specify WithAddr, it defaults to ":9000".
func ListenAndServeWithContextOptions(ctx context.Context, setup SetupFunc, opts ...ServeOption) error {
	return ListenAndServeWithContext(ctx, "", setup, opts...)
}

// Serve handles accepted connections from an existing listener.
func Serve(listener net.Listener, setup SetupFunc) error {
	return ServeWithOptions(listener, setup)
}

// ServeWithOptions handles accepted connections from an existing listener with options.
func ServeWithOptions(listener net.Listener, setup SetupFunc, opts ...ServeOption) error {
	return ServeWithContext(context.Background(), listener, setup, opts...)
}

// ServeWithContext handles accepted connections from an existing listener with options and a cancelable context.
//
// When ctx is canceled, the listener will be closed and Serve will return.
func ServeWithContext(ctx context.Context, listener net.Listener, setup SetupFunc, opts ...ServeOption) error {
	if setup == nil {
		return ErrNoSetup
	}
	if ctx == nil {
		ctx = context.Background()
	}

	so := applyServeOptions(opts)

	router := NewRouter()
	if err := setup(router); err != nil {
		return err
	}
	closeOnDone(ctx.Done(), listener)
	return acceptLoop(ctx, listener, router, so)
}

type closer interface {
	Close() error
}

func closeOnDone(done <-chan struct{}, c closer) {
	if done == nil || c == nil {
		return
	}
	go func() {
		<-done
		_ = c.Close()
	}()
}

func closeConnOnDone(ctx context.Context, c net.Conn) func() {
	done := ctx.Done()
	if done == nil {
		return nil
	}
	stop := make(chan struct{})
	go func() {
		select {
		case <-done:
			_ = c.Close()
		case <-stop:
		}
	}()
	return func() { close(stop) }
}

func acceptLoop(ctx context.Context, listener net.Listener, router *Router, so serveOptions) error {
	acceptBackoff := 5 * time.Millisecond
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			if isRetryableAcceptError(err) {
				log.Printf("accept retryable error: %v", err)
				time.Sleep(acceptBackoff)
				acceptBackoff = nextAcceptBackoff(acceptBackoff)
				continue
			}
			return err
		}
		acceptBackoff = 5 * time.Millisecond
		go serveConn(ctx, conn, router, so)
	}
}

func isRetryableAcceptError(err error) bool {
	if err == nil {
		return false
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return true
	}

	switch {
	case errors.Is(err, syscall.EINTR):
		return true
	case errors.Is(err, syscall.EAGAIN):
		return true
	case errors.Is(err, syscall.ECONNABORTED):
		return true
	case errors.Is(err, syscall.EMFILE):
		return true
	case errors.Is(err, syscall.ENFILE):
		return true
	case errors.Is(err, syscall.ENOBUFS):
		return true
	case errors.Is(err, syscall.ENOMEM):
		return true
	default:
		return false
	}
}

func nextAcceptBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > time.Second {
		return time.Second
	}
	return next
}

func serveConn(ctx context.Context, c net.Conn, router *Router, so serveOptions) {
	defer c.Close()
	stop := closeConnOnDone(ctx, c)
	if stop != nil {
		defer stop()
	}
	if err := handleConn(ctx, c, router, so.idleTimeout, so.writeTimeout); err != nil && !isBenignConnError(err) {
		log.Printf("conn error: %v", err)
	}
}

func isBenignConnError(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	if errors.Is(err, syscall.EPIPE) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	return false
}

// BridgeProtocolHandler returns a protocol.Handler that forwards the payload to fn.
// It preserves RequestID and command on the response.
func BridgeProtocolHandler(cmd uint16, fn func(context.Context, []byte) ([]byte, error)) Handler {
	return func(ctx context.Context, m *protocol.Message) (*protocol.Message, error) {
		out, err := fn(ctx, m.Payload)
		if err != nil {
			return nil, err
		}
		return &protocol.Message{
			Command:   cmd,
			RequestID: m.RequestID,
			Payload:   out,
		}, nil
	}
}
