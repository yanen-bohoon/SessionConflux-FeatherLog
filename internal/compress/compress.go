package compress

import (
	"bytes"

	"github.com/klauspost/compress/zstd"
)

// Compress compresses data with zstd at the given level (1-22).
func Compress(data []byte, level int) ([]byte, error) {
	if level < 1 || level > 22 {
		level = 3
	}
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.EncoderLevel(level)))
	if err != nil {
		return nil, err
	}
	defer encoder.Close()
	return encoder.EncodeAll(data, nil), nil
}

// Decompress decompresses zstd data.
func Decompress(data []byte) ([]byte, error) {
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, err
	}
	defer decoder.Close()
	return decoder.DecodeAll(data, nil)
}

// CompressToBuffer compresses data into the provided buffer (avoids alloc).
func CompressToBuffer(data []byte, level int, buf *bytes.Buffer) error {
	if level < 1 || level > 22 {
		level = 3
	}
	encoder, err := zstd.NewWriter(buf, zstd.WithEncoderLevel(zstd.EncoderLevel(level)))
	if err != nil {
		return err
	}
	if _, err := encoder.Write(data); err != nil {
		encoder.Close()
		return err
	}
	return encoder.Close()
}
