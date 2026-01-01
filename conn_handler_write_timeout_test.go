package novagate

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/gogogo1024/novagate/protocol"
)

func TestHandleConnWriteTimeoutReturnsError(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()

	// Never read from client. Server write should block and then time out.
	r := NewRouter()
	r.Register(protocol.CmdPing, func(ctx context.Context, m *protocol.Message) (*protocol.Message, error) {
		return &protocol.Message{Command: m.Command, RequestID: m.RequestID, Payload: []byte("pong")}, nil
	})

	done := make(chan error, 1)
	go func() {
		done <- handleConn(context.Background(), server, r, 500*time.Millisecond, 50*time.Millisecond)
	}()

	// Write a request frame from the client side.
	msgBytes, err := protocol.EncodeMessage(&protocol.Message{Command: protocol.CmdPing, RequestID: 1, Payload: []byte("ping")})
	if err != nil {
		t.Fatalf("EncodeMessage: %v", err)
	}
	flags, body, err := protocol.EncodeFrameBody(0, msgBytes)
	if err != nil {
		t.Fatalf("EncodeFrameBody: %v", err)
	}
	frameBytes := protocol.Encode(&protocol.Frame{Flags: flags, Body: body})

	if _, err := client.Write(frameBytes); err != nil {
		t.Fatalf("client write request: %v", err)
	}

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected write timeout error, got nil")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("expected handler to exit due to write timeout")
	}
}
