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

func TestFrameMagicValidation(t *testing.T) {
	// Build a frame with invalid magic (not 0xCAFE).
	buf := make([]byte, FrameHeaderLen+5)
	// Wrong magic: 0xDEAD instead of 0xCAFE
	copy(buf[0:2], []byte{0xDE, 0xAD})
	buf[2] = FrameVersion
	buf[3] = 0
	// Length = 5
	copy(buf[4:8], []byte{0x00, 0x00, 0x00, 0x05})
	copy(buf[8:], []byte{1, 2, 3, 4, 5})

	_, _, err := Decode(buf)
	if err == nil {
		t.Fatalf("expected error for invalid magic, got nil")
	}
}

func TestFrameVersionValidation(t *testing.T) {
	// Build a frame with unsupported version.
	buf := make([]byte, FrameHeaderLen+5)
	copy(buf[0:2], []byte{0xCA, 0xFE})
	buf[2] = 99 // Unsupported version
	buf[3] = 0
	copy(buf[4:8], []byte{0x00, 0x00, 0x00, 0x05})
	copy(buf[8:], []byte{1, 2, 3, 4, 5})

	_, _, err := Decode(buf)
	if err == nil {
		t.Fatalf("expected error for unsupported version, got nil")
	}
}

func TestFrameLengthLimit(t *testing.T) {
	// Build a frame with body length > MaxFrameBody.
	buf := make([]byte, FrameHeaderLen)
	copy(buf[0:2], []byte{0xCA, 0xFE})
	buf[2] = FrameVersion
	buf[3] = 0
	// Length = MaxFrameBody + 1 (too large)
	oversize := MaxFrameBody + 1
	copy(buf[4:8], []byte{
		byte((oversize >> 24) & 0xFF),
		byte((oversize >> 16) & 0xFF),
		byte((oversize >> 8) & 0xFF),
		byte(oversize & 0xFF),
	})

	_, _, err := Decode(buf)
	if err == nil {
		t.Fatalf("expected error for frame too large, got nil")
	}
}

func TestEncryptedFlagRejection(t *testing.T) {
	// Verify that FlagEncrypted is rejected.
	err := ValidateFlags(FlagEncrypted)
	if err == nil {
		t.Fatalf("expected error for FlagEncrypted, got nil")
	}
	if err != ErrUnsupportedFrameFlags {
		t.Fatalf("expected ErrUnsupportedFrameFlags, got %v", err)
	}
}

func TestOneWayFlagPreservation(t *testing.T) {
	// Verify that OneWay flag is preserved during encode/decode.
	msgBytes := []byte{0x01, 0x02, 0x03, 0x04}

	flags, encoded, err := EncodeFrameBody(FlagOneWay, msgBytes)
	if err != nil {
		t.Fatalf("EncodeFrameBody error: %v", err)
	}
	if flags&FlagOneWay == 0 {
		t.Fatalf("FlagOneWay not set in returned flags")
	}

	// Decode should preserve the flag in the Frame
	frame := &Frame{Flags: flags, Body: encoded}
	decoded, err := DecodeFrameBody(frame)
	if err != nil {
		t.Fatalf("DecodeFrameBody error: %v", err)
	}
	if !bytes.Equal(decoded, msgBytes) {
		t.Fatalf("decoded body mismatch: got %v, want %v", decoded, msgBytes)
	}

	// Flags should still be readable from frame
	if frame.Flags&FlagOneWay == 0 {
		t.Fatalf("FlagOneWay not preserved in Frame")
	}
}

func TestRequestID_MultipleMessages(t *testing.T) {
	// Verify that multiple messages can carry different RequestIDs.
	requestIDs := []uint64{1, 100, 999999, 18446744073709551615} // Max uint64

	for _, rid := range requestIDs {
		msg := &Message{
			Command:   0x0001,
			RequestID: rid,
			Payload:   []byte("test"),
		}

		wire, err := EncodeMessage(msg)
		if err != nil {
			t.Fatalf("EncodeMessage for RequestID=%d: %v", rid, err)
		}

		decoded, err := DecodeMessage(wire)
		if err != nil {
			t.Fatalf("DecodeMessage for RequestID=%d: %v", rid, err)
		}

		if decoded.RequestID != rid {
			t.Fatalf("RequestID mismatch: got %d, want %d", decoded.RequestID, rid)
		}
	}
}

func TestCompressionAndOneWayTogether(t *testing.T) {
	// Verify that compression and OneWay flags can coexist (OneWay just skips response).
	original := []byte("compressed_oneway_payload")

	flags := FlagCompressed | FlagOneWay
	encoded, err := encodeFrameBodyWithCompression(original)
	if err != nil {
		t.Fatalf("encodeFrameBodyWithCompression error: %v", err)
	}

	// Simulate receiving with both flags
	frame := &Frame{Flags: flags, Body: encoded}
	decoded, err := DecodeFrameBody(frame)
	if err != nil {
		t.Fatalf("DecodeFrameBody error: %v", err)
	}

	if !bytes.Equal(decoded, original) {
		t.Fatalf("decoded body mismatch: got %v, want %v", decoded, original)
	}

	// Both flags should be readable
	if frame.Flags&FlagCompressed == 0 {
		t.Fatalf("FlagCompressed not preserved")
	}
	if frame.Flags&FlagOneWay == 0 {
		t.Fatalf("FlagOneWay not preserved")
	}
}

// Helper: encodeFrameBodyWithCompression wraps EncodeFrameBody for compression.
func encodeFrameBodyWithCompression(payload []byte) ([]byte, error) {
	_, body, err := EncodeFrameBody(FlagCompressed, payload)
	return body, err
}
