package protocol

import (
	"bytes"
	"testing"
)

func TestFrameEncodeDecodeRoundTrip(t *testing.T) {
	in := &Frame{
		// Version=0 means Encode() should default to FrameVersion.
		Flags: FlagOneWay,
		Body:  []byte("hello"),
	}

	wire := Encode(in)
	out, n, err := Decode(wire)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if n != len(wire) {
		t.Fatalf("Decode consumed %d bytes, want %d", n, len(wire))
	}
	if out == nil {
		t.Fatalf("Decode returned nil frame")
	}
	if out.Version != FrameVersion {
		t.Fatalf("Version=%d, want %d", out.Version, FrameVersion)
	}
	if out.Flags != in.Flags {
		t.Fatalf("Flags=0x%02X, want 0x%02X", out.Flags, in.Flags)
	}
	if !bytes.Equal(out.Body, in.Body) {
		t.Fatalf("Body mismatch: got %q, want %q", out.Body, in.Body)
	}
}

func TestMessageEncodeDecodeRoundTrip(t *testing.T) {
	in := &Message{
		Command:   0x0102,
		RequestID: 123456789,
		Payload:   []byte{0x01, 0x02, 0x03, 0x04},
	}

	wire, err := EncodeMessage(in)
	if err != nil {
		t.Fatalf("EncodeMessage error: %v", err)
	}
	out, err := DecodeMessage(wire)
	if err != nil {
		t.Fatalf("DecodeMessage error: %v", err)
	}
	if out.Command != in.Command {
		t.Fatalf("Command=0x%04X, want 0x%04X", out.Command, in.Command)
	}
	if out.RequestID != in.RequestID {
		t.Fatalf("RequestID=%d, want %d", out.RequestID, in.RequestID)
	}
	if !bytes.Equal(out.Payload, in.Payload) {
		t.Fatalf("Payload mismatch: got %v, want %v", out.Payload, in.Payload)
	}
}

func TestCompressedFlagRoundTrip(t *testing.T) {
	original := []byte("this payload should round-trip through gzip")

	flags, encoded, err := EncodeFrameBody(FlagCompressed, original)
	if err != nil {
		t.Fatalf("EncodeFrameBody error: %v", err)
	}
	if flags&FlagCompressed == 0 {
		t.Fatalf("FlagCompressed not set in returned flags")
	}

	decoded, err := DecodeFrameBody(&Frame{Flags: flags, Body: encoded})
	if err != nil {
		t.Fatalf("DecodeFrameBody error: %v", err)
	}
	if !bytes.Equal(decoded, original) {
		t.Fatalf("decoded body mismatch: got %q, want %q", decoded, original)
	}
}
