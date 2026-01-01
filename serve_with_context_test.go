package novagate

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestServeWithContextCancelStopsServe(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ServeWithContext(ctx, listener, func(r *Router) error { return nil })
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ServeWithContext returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for ServeWithContext to stop")
	}
}
