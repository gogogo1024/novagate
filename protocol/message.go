package protocol

import (
	"encoding/binary"
	"errors"
)

// MessageHeaderLen is the fixed header length in bytes:
// Command(uint16) + RequestID(uint64).
const MessageHeaderLen = 2 + 8

type Message struct {
	Command   uint16
	RequestID uint64
	Payload   []byte
}

func EncodeMessage(m *Message) ([]byte, error) {
	buf := make([]byte, MessageHeaderLen+len(m.Payload))
	binary.BigEndian.PutUint16(buf[0:2], m.Command)
	binary.BigEndian.PutUint64(buf[2:10], m.RequestID)
	copy(buf[10:], m.Payload)
	return buf, nil
}

func DecodeMessage(data []byte) (*Message, error) {
	if len(data) < MessageHeaderLen {
		return nil, errors.New("message too short")
	}
	m := &Message{
		Command:   binary.BigEndian.Uint16(data[0:2]),
		RequestID: binary.BigEndian.Uint64(data[2:10]),
		Payload:   data[10:],
	}
	return m, nil
}
