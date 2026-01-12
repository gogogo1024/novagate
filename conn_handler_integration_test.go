package novagate

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"github.com/gogogo1024/novagate/protocol"
)

// TestHandleConn_BasicEcho tests basic request/response flow using TCP listener.
func TestHandleConn_BasicEcho(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer listener.Close()

	r := NewRouter()
	r.Register(protocol.CmdPing, func(ctx context.Context, m *protocol.Message) (*protocol.Message, error) {
		return &protocol.Message{
			Command:   m.Command,
			RequestID: m.RequestID,
			Payload:   m.Payload,
		}, nil
	})

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = handleConn(context.Background(), conn, r, 5*time.Second, 5*time.Second)
	}()

	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close()

	// Send request
	payload := []byte("ping_test")
	msg := &protocol.Message{Command: protocol.CmdPing, RequestID: 42, Payload: payload}
	msgBytes, _ := protocol.EncodeMessage(msg)
	frameFlags, frameBody, _ := protocol.EncodeFrameBody(0, msgBytes)
	frameBytes := protocol.Encode(&protocol.Frame{Flags: frameFlags, Body: frameBody})

	if _, err := client.Write(frameBytes); err != nil {
		t.Fatalf("Write request: %v", err)
	}

	// Read response
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	tmp := make([]byte, 256)
	n, err := client.Read(tmp)
	if err != nil {
		t.Fatalf("Read: err=%v", err)
	}

	frame, _, _ := protocol.Decode(tmp[:n])
	respBody, _ := protocol.DecodeFrameBody(frame)
	respMsg, _ := protocol.DecodeMessage(respBody)

	if respMsg.RequestID != 42 {
		t.Fatalf("RequestID: got %d, want 42", respMsg.RequestID)
	}
	if !bytes.Equal(respMsg.Payload, payload) {
		t.Fatalf("Payload: got %v, want %v", respMsg.Payload, payload)
	}
}

// TestHandleConn_OneWayNoResponse tests that OneWay flag prevents response.
func TestHandleConn_OneWayNoResponse(t *testing.T) {
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()

	r := NewRouter()
	r.Register(protocol.CmdPing, func(ctx context.Context, m *protocol.Message) (*protocol.Message, error) {
		return &protocol.Message{Command: m.Command, RequestID: m.RequestID, Payload: []byte("should_not_send")}, nil
	})

	go func() {
		conn, _ := listener.Accept()
		defer conn.Close()
		_ = handleConn(context.Background(), conn, r, 5*time.Second, 5*time.Second)
	}()

	client, _ := net.Dial("tcp", listener.Addr().String())
	defer client.Close()

	// Send OneWay request
	msg := &protocol.Message{Command: protocol.CmdPing, RequestID: 1, Payload: []byte("ping")}
	msgBytes, _ := protocol.EncodeMessage(msg)
	frameFlags, frameBody, _ := protocol.EncodeFrameBody(protocol.FlagOneWay, msgBytes)
	frameBytes := protocol.Encode(&protocol.Frame{Flags: frameFlags, Body: frameBody})
	client.Write(frameBytes)

	// Verify no response comes back
	client.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	buf := make([]byte, 1024)
	n, err := client.Read(buf)

	if err == nil && n > 0 {
		t.Fatalf("expected no response for OneWay request, got %d bytes", n)
	}
}

// TestHandleConn_CompressionRoundTrip tests compression encoding/decoding.
func TestHandleConn_CompressionRoundTrip(t *testing.T) {
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()

	r := NewRouter()
	r.Register(protocol.CmdPing, func(ctx context.Context, m *protocol.Message) (*protocol.Message, error) {
		return &protocol.Message{Command: m.Command, RequestID: m.RequestID, Payload: m.Payload}, nil
	})

	go func() {
		conn, _ := listener.Accept()
		defer conn.Close()
		_ = handleConn(context.Background(), conn, r, 5*time.Second, 5*time.Second)
	}()

	client, _ := net.Dial("tcp", listener.Addr().String())
	defer client.Close()

	// Send compressed request
	payload := []byte("compression_test_payload_with_some_content")
	msg := &protocol.Message{Command: protocol.CmdPing, RequestID: 100, Payload: payload}
	msgBytes, _ := protocol.EncodeMessage(msg)
	frameFlags, frameBody, _ := protocol.EncodeFrameBody(protocol.FlagCompressed, msgBytes)
	frameBytes := protocol.Encode(&protocol.Frame{Flags: frameFlags, Body: frameBody})
	client.Write(frameBytes)

	// Read response
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	tmp := make([]byte, 512)
	n, _ := client.Read(tmp)

	frame, _, _ := protocol.Decode(tmp[:n])
	respBody, _ := protocol.DecodeFrameBody(frame)
	respMsg, _ := protocol.DecodeMessage(respBody)

	if respMsg.RequestID != 100 {
		t.Fatalf("RequestID: got %d, want 100", respMsg.RequestID)
	}
	if !bytes.Equal(respMsg.Payload, payload) {
		t.Fatalf("Payload mismatch")
	}
}

// TestHandleConn_MultipleSequentialRequests tests handling multiple requests.
func TestHandleConn_MultipleSequentialRequests(t *testing.T) {
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()

	r := NewRouter()
	r.Register(protocol.CmdPing, func(ctx context.Context, m *protocol.Message) (*protocol.Message, error) {
		return &protocol.Message{Command: m.Command, RequestID: m.RequestID, Payload: m.Payload}, nil
	})

	go func() {
		conn, _ := listener.Accept()
		defer conn.Close()
		_ = handleConn(context.Background(), conn, r, 5*time.Second, 5*time.Second)
	}()

	client, _ := net.Dial("tcp", listener.Addr().String())
	defer client.Close()

	// Send and verify 3 requests
	for i := uint64(1); i <= 3; i++ {
		payload := []byte("request_" + string(rune(48+i)))
		msg := &protocol.Message{Command: protocol.CmdPing, RequestID: i, Payload: payload}
		msgBytes, _ := protocol.EncodeMessage(msg)
		_, frameBody, _ := protocol.EncodeFrameBody(0, msgBytes)
		frameBytes := protocol.Encode(&protocol.Frame{Body: frameBody})
		client.Write(frameBytes)

		client.SetReadDeadline(time.Now().Add(2 * time.Second))
		tmp := make([]byte, 256)
		n, _ := client.Read(tmp)

		frame, _, _ := protocol.Decode(tmp[:n])
		respBody, _ := protocol.DecodeFrameBody(frame)
		respMsg, _ := protocol.DecodeMessage(respBody)

		if respMsg.RequestID != i {
			t.Fatalf("Request %d: RequestID mismatch", i)
		}
	}
}

// TestHandleConn_RequestIDMatching tests that response RequestID matches request.
func TestHandleConn_RequestIDMatching(t *testing.T) {
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()

	r := NewRouter()
	r.Register(protocol.CmdPing, func(ctx context.Context, m *protocol.Message) (*protocol.Message, error) {
		// Intentionally return with RequestID=0 to test auto-fill
		return &protocol.Message{Command: m.Command, Payload: m.Payload}, nil
	})

	go func() {
		conn, _ := listener.Accept()
		defer conn.Close()
		_ = handleConn(context.Background(), conn, r, 5*time.Second, 5*time.Second)
	}()

	client, _ := net.Dial("tcp", listener.Addr().String())
	defer client.Close()

	// Send with specific RequestID
	msg := &protocol.Message{Command: protocol.CmdPing, RequestID: 999, Payload: []byte("test")}
	msgBytes, _ := protocol.EncodeMessage(msg)
	_, frameBody, _ := protocol.EncodeFrameBody(0, msgBytes)
	frameBytes := protocol.Encode(&protocol.Frame{Body: frameBody})
	client.Write(frameBytes)

	// Should get response with RequestID=999 (auto-filled by handleFrame)
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	tmp := make([]byte, 256)
	n, _ := client.Read(tmp)

	frame, _, _ := protocol.Decode(tmp[:n])
	respBody, _ := protocol.DecodeFrameBody(frame)
	respMsg, _ := protocol.DecodeMessage(respBody)

	if respMsg.RequestID != 999 {
		t.Fatalf("expected RequestID=999, got %d", respMsg.RequestID)
	}
}
