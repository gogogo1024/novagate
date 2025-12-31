package novagate

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/gogogo1024/novagate/protocol"
)

func HandleConn(ctx context.Context, conn net.Conn, router *Router) error {
	if router == nil {
		return errors.New("novagate: nil router")
	}

	state := &connHandlerState{
		cc:  NewConnContext(),
		buf: make([]byte, 0, 8*1024),
		tmp: make([]byte, 4*1024),
	}
	defer state.cc.Release(len(state.buf))

	for {
		if err := readIntoBuffer(conn, state); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if err := processBufferedFrames(ctx, conn, state, router); err != nil {
			return err
		}
	}
}

type connHandlerState struct {
	cc  *ConnContext
	buf []byte
	tmp []byte
}

func readIntoBuffer(conn net.Conn, state *connHandlerState) error {
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

	n, err := conn.Read(state.tmp)
	if n > 0 {
		if !state.cc.Reserve(n) {
			return errors.New("connection buffer quota exceeded")
		}
		state.buf = append(state.buf, state.tmp[:n]...)
	}
	return err
}

func processBufferedFrames(ctx context.Context, conn net.Conn, state *connHandlerState, router *Router) error {
	consumed := 0

	for {
		frame, frameLen, err := protocol.Decode(state.buf[consumed:])
		if err != nil {
			return err
		}
		if frame == nil {
			break
		}

		if err := handleFrame(ctx, conn, state, router, frame); err != nil {
			return err
		}
		consumed += frameLen
	}

	if consumed > 0 {
		state.cc.Release(consumed)
		copy(state.buf, state.buf[consumed:])
		state.buf = state.buf[:len(state.buf)-consumed]
	}
	return nil
}

func handleFrame(ctx context.Context, conn net.Conn, state *connHandlerState, router *Router, frame *protocol.Frame) error {
	oneWay := (frame.Flags & protocol.FlagOneWay) != 0

	if !state.cc.Allow() {
		return errors.New("rate limit exceeded")
	}

	body, err := protocol.DecodeFrameBody(frame)
	if err != nil {
		return err
	}

	msg, err := protocol.DecodeMessage(body)
	if err != nil {
		return err
	}

	resp, err := router.Dispatch(ctx, msg)
	if err != nil {
		return err
	}
	if oneWay || resp == nil {
		return nil
	}

	if resp.RequestID == 0 {
		resp.RequestID = msg.RequestID
	}

	respBytes, err := protocol.EncodeMessage(resp)
	if err != nil {
		return err
	}

	outFlags := frame.Flags & protocol.FlagCompressed
	outFlags, outBody, err := protocol.EncodeFrameBody(outFlags, respBytes)
	if err != nil {
		return err
	}

	out := protocol.Encode(&protocol.Frame{Flags: outFlags, Body: outBody})
	return writeAll(conn, out)
}

func writeAll(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}
