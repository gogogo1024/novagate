package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	// FrameMagic is the 2-byte magic number at the start of every frame.
	FrameMagic uint16 = 0xCAFE
	// FrameVersion is the current supported protocol frame version.
	FrameVersion uint8 = 1

	// FrameHeaderLen is the fixed header length in bytes.
	FrameHeaderLen = 8
	// MaxFrameBody is the maximum allowed frame body size in bytes.
	MaxFrameBody = 1024 * 1024 // 1MB
)

const (
	FlagCompressed uint8 = 1 << 0
	FlagEncrypted  uint8 = 1 << 1
	FlagOneWay     uint8 = 1 << 2
)

type Frame struct {
	Version uint8
	Flags   uint8
	Body    []byte
}

func Decode(buf []byte) (*Frame, int, error) {
	if len(buf) < FrameHeaderLen {
		return nil, 0, nil
	}

	magic := binary.BigEndian.Uint16(buf[0:2])
	if magic != FrameMagic {
		return nil, 0, fmt.Errorf("invalid frame magic: 0x%04X", magic)
	}

	version := buf[2]
	if version != FrameVersion {
		return nil, 0, fmt.Errorf("unsupported frame version: %d", version)
	}

	flags := buf[3]
	length := binary.BigEndian.Uint32(buf[4:8])
	if length > MaxFrameBody {
		return nil, 0, errors.New("frame too large")
	}

	totalLen := int(length) + FrameHeaderLen
	if len(buf) < totalLen {
		return nil, 0, nil
	}

	f := &Frame{
		Version: version,
		Flags:   flags,
		Body:    buf[FrameHeaderLen:totalLen],
	}

	return f, totalLen, nil
}

func Encode(f *Frame) []byte {
	bodyLen := len(f.Body)
	if bodyLen > int(MaxFrameBody) {
		panic("frame body too large")
	}

	buf := make([]byte, FrameHeaderLen+bodyLen)
	binary.BigEndian.PutUint16(buf[0:2], FrameMagic)
	version := f.Version
	if version == 0 {
		version = FrameVersion
	}
	buf[2] = version
	buf[3] = f.Flags
	binary.BigEndian.PutUint32(buf[4:8], uint32(bodyLen))
	copy(buf[FrameHeaderLen:], f.Body[:bodyLen])
	return buf
}
