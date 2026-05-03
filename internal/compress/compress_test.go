package compress

import (
	"bytes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	original := []byte(`{"role":"user","content":"hello world"}` + "\n" +
		`{"role":"assistant","content":"hi there"}` + "\n")

	for _, level := range []int{1, 3} {
		compressed, err := Compress(original, level)
		if err != nil {
			t.Fatalf("compress level %d: %v", level, err)
		}
		if len(compressed) == 0 {
			t.Errorf("level %d: compressed data is empty", level)
		}

		decompressed, err := Decompress(compressed)
		if err != nil {
			t.Fatalf("decompress level %d: %v", level, err)
		}
		if !bytes.Equal(decompressed, original) {
			t.Errorf("level %d: round-trip mismatch", level)
		}
	}
}

func TestCompress_DefaultLevel(t *testing.T) {
	// Out of range levels should default to 3.
	compressed, err := Compress([]byte("test"), 0)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	decompressed, err := Decompress(compressed)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if string(decompressed) != "test" {
		t.Errorf("got %q, want %q", decompressed, "test")
	}
}

func TestCompress_HighLevel(t *testing.T) {
	compressed, err := Compress([]byte("test"), 99)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	decompressed, err := Decompress(compressed)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if string(decompressed) != "test" {
		t.Errorf("got %q, want %q", decompressed, "test")
	}
}

func TestCompress_EmptyInput(t *testing.T) {
	compressed, err := Compress([]byte{}, 3)
	if err != nil {
		t.Fatalf("compress empty: %v", err)
	}
	decompressed, err := Decompress(compressed)
	if err != nil {
		t.Fatalf("decompress empty: %v", err)
	}
	if len(decompressed) != 0 {
		t.Errorf("expected empty, got %d bytes", len(decompressed))
	}
}

func TestDecompress_Corrupted(t *testing.T) {
	_, err := Decompress([]byte("not zstd data"))
	if err == nil {
		t.Error("expected error for corrupted data")
	}
}

func TestCompress_LargerPayload(t *testing.T) {
	// Generate 100KB of semi-repetitive data.
	var buf bytes.Buffer
	for i := 0; i < 1000; i++ {
		buf.WriteString(`{"role":"user","content":"message number `)
	}
	original := buf.Bytes()

	compressed, err := Compress(original, 3)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	// Repetitive data should compress well.
	if len(compressed) >= len(original) {
		t.Errorf("compressed size %d >= original %d", len(compressed), len(original))
	}

	decompressed, err := Decompress(compressed)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if !bytes.Equal(decompressed, original) {
		t.Error("larger payload round-trip mismatch")
	}
}
