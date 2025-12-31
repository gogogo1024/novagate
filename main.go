package novagate

import (
	"context"
	"errors"
	"log"
	"net"

	"github.com/gogogo1024/novagate/protocol"
)

// SetupFunc is an injection point for registering command tables and handlers.
// It is called once before serving starts.
type SetupFunc func(r *Router) error

var ErrNoSetup = errors.New("novagate: setup is required")

// ListenAndServe starts a TCP listener on addr and serves the Novagate protocol.
// The caller must provide setup to register command mappings and handlers.
func ListenAndServe(addr string, setup SetupFunc) error {
	if addr == "" {
		addr = ":9000"
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return Serve(listener, setup)
}

// Serve handles accepted connections from an existing listener.
func Serve(listener net.Listener, setup SetupFunc) error {
	if setup == nil {
		return ErrNoSetup
	}
	router := NewRouter()
	if err := setup(router); err != nil {
		return err
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			ctx := context.Background()
			if err := HandleConn(ctx, c, router); err != nil {
				log.Printf("conn error: %v", err)
			}
		}(conn)
	}
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
