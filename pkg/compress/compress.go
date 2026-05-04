package compress

import (
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
