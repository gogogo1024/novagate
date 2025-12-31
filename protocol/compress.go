package protocol

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
)

var ErrUnsupportedFrameFlags = errors.New("unsupported frame flags")

func ValidateFlags(flags uint8) error {
	if flags&FlagEncrypted != 0 {
		return ErrUnsupportedFrameFlags
	}
	return nil
}

// DecodeFrameBody validates flags and returns a decoded body for Message decoding.
// If FlagCompressed is set, it will gzip-decompress the body.
func DecodeFrameBody(f *Frame) ([]byte, error) {
	if f == nil {
		return nil, errors.New("nil frame")
	}
	if err := ValidateFlags(f.Flags); err != nil {
		return nil, err
	}
	if f.Flags&FlagCompressed == 0 {
		return f.Body, nil
	}
	return gzipDecompress(f.Body, int(MaxFrameBody))
}

// EncodeFrameBody validates flags and returns an encoded body for Frame writing.
// If FlagCompressed is set, it will gzip-compress the body.
func EncodeFrameBody(flags uint8, body []byte) (uint8, []byte, error) {
	if err := ValidateFlags(flags); err != nil {
		return 0, nil, err
	}
	if flags&FlagCompressed == 0 {
		return flags, body, nil
	}
	out, err := gzipCompress(body)
	if err != nil {
		return 0, nil, err
	}
	return flags, out, nil
}

func gzipCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gzipDecompress(data []byte, maxOutput int) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	if maxOutput <= 0 {
		return nil, errors.New("invalid maxOutput")
	}

	limited := io.LimitReader(zr, int64(maxOutput)+1)
	out, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(out) > maxOutput {
		return nil, errors.New("decompressed body too large")
	}
	return out, nil
}
