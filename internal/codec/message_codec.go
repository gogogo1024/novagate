package codec

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/gogogo1024/novagate/protocol"

	"github.com/cloudwego/kitex/pkg/remote"
)

type MessageCodec struct{}

func (c *MessageCodec) Name() string { return "novagate" }

func (c *MessageCodec) Encode(
	ctx context.Context,
	msg remote.Message,
	out remote.ByteBuffer,
) error {

	rpcInfo := msg.RPCInfo()
	inv := rpcInfo.Invocation()

	fullMethod := fmt.Sprintf("%s.%s", inv.ServiceName(), inv.MethodName())
	cmd, err := protocol.MapMethodToCommand(fullMethod)
	if err != nil {
		return err
	}

	var payload []byte
	switch v := msg.Data().(type) {
	case []byte:
		payload = v
	case *[]byte:
		if v != nil {
			payload = *v
		}
	case nil:
		payload = nil
	default:
		return fmt.Errorf("unsupported kitex message data type: %T", v)
	}

	myMsg := &protocol.Message{
		Command:   cmd,
		RequestID: uint64(inv.SeqID()),
		Payload:   payload,
	}

	data, err := protocol.EncodeMessage(myMsg)
	if err != nil {
		return err
	}

	flags := uint8(0)
	if tags := msg.Tags(); tags != nil {
		flags = parseFlags(tags["novagate.flags"])
	}

	frameFlags, frameBody, err := protocol.EncodeFrameBody(flags, data)
	if err != nil {
		return err
	}

	frameBytes := protocol.Encode(&protocol.Frame{Flags: frameFlags, Body: frameBody})
	_, err = out.WriteBinary(frameBytes)
	if err != nil {
		return err
	}
	return nil
}

func (c *MessageCodec) Decode(
	ctx context.Context,
	msg remote.Message,
	in remote.ByteBuffer,
) error {
	readable := in.ReadableLen()
	if readable <= 0 {
		return errors.New("empty input")
	}
	buf := make([]byte, readable)
	n, err := in.ReadBinary(buf)
	if err != nil {
		return err
	}
	buf = buf[:n]

	frame, _, err := protocol.Decode(buf)
	if err != nil {
		return err
	}
	if frame == nil {
		return errors.New("incomplete frame")
	}

	body, err := protocol.DecodeFrameBody(frame)
	if err != nil {
		return err
	}

	myMsg, err := protocol.DecodeMessage(body)
	if err != nil {
		return err
	}

	// Best-effort: populate msg.Data() when it's a pointer type.
	switch d := msg.Data().(type) {
	case *[]byte:
		if d != nil {
			*d = myMsg.Payload
		}
	case *interface{}:
		if d != nil {
			*d = myMsg.Payload
		}
	}
	msg.SetPayloadLen(len(myMsg.Payload))

	// Preserve protocol metadata for upper layers.
	tags := msg.Tags()
	if tags != nil {
		tags["novagate.command"] = myMsg.Command
		tags["novagate.request_id"] = myMsg.RequestID
		tags["novagate.flags"] = frame.Flags
	}

	return nil
}

func parseFlags(v any) uint8 {
	switch x := v.(type) {
	case uint8:
		return x
	case uint16:
		return uint8(x)
	case uint32:
		return uint8(x)
	case uint64:
		return uint8(x)
	case int:
		return uint8(x)
	case int32:
		return uint8(x)
	case int64:
		return uint8(x)
	case string:
		// Accept "0x.." or decimal.
		if u, err := strconv.ParseUint(x, 0, 8); err == nil {
			return uint8(u)
		}
		return 0
	default:
		return 0
	}
}
