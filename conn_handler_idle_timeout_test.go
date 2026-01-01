package novagate

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestHandleConnIdleTimeoutReturnsNil(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()

	r := NewRouter()

	done := make(chan error, 1)
	go func() {
		done <- handleConn(context.Background(), server, r, 50*time.Millisecond, 0)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil error on idle timeout, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("expected connection handler to exit on idle timeout")
	}
}
