package bundle

import (
	"testing"
)

func TestPackUnpack_RoundTrip(t *testing.T) {
	sessions := map[string][]byte{
		"mac-studio/claude/sess-1.jsonl": []byte(`{"role":"user","content":"hello"}` + "\n" +
			`{"role":"assistant","content":"hi"}` + "\n"),
		"mac-studio/claude/sess-2.jsonl": []byte(`{"role":"user","content":"test"}` + "\n"),
		"thinkpad/codex/sess-a.jsonl":    []byte(`{"role":"user","content":"codex session"}` + "\n" +
			`{"role":"assistant","content":"response"}` + "\n" +
			`{"role":"user","content":"follow up"}` + "\n"),
	}

	archive, err := Pack(sessions, 3)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	if len(archive) == 0 {
		t.Fatal("archive is empty")
	}

	entries, err := Unpack(archive)
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	if len(entries) != len(sessions) {
		t.Fatalf("got %d entries, want %d", len(entries), len(sessions))
	}

	for name, expected := range sessions {
		got, ok := entries[name]
		if !ok {
			t.Errorf("missing entry: %s", name)
			continue
		}
		if string(got) != string(expected) {
			t.Errorf("entry %s: content mismatch", name)
		}
	}
}

func TestPackUnpack_SingleEntry(t *testing.T) {
	sessions := map[string][]byte{
		"host/claude/session.jsonl": []byte("single line\n"),
	}

	archive, err := Pack(sessions, 1)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	entries, err := Unpack(archive)
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
}

func TestPackUnpack_Empty(t *testing.T) {
	archive, err := Pack(map[string][]byte{}, 3)
	if err != nil {
		t.Fatalf("pack empty: %v", err)
	}

	entries, err := Unpack(archive)
	if err != nil {
		t.Fatalf("unpack empty: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries from empty pack, want 0", len(entries))
	}
}

func TestPackCompressionLevels(t *testing.T) {
	data := map[string][]byte{
		"host/claude/s.jsonl": []byte("test data\n"),
	}

	for _, level := range []int{1, 3} {
		archive, err := Pack(data, level)
		if err != nil {
			t.Fatalf("pack level %d: %v", level, err)
		}
		entries, err := Unpack(archive)
		if err != nil {
			t.Fatalf("unpack level %d: %v", level, err)
		}
		if len(entries) != 1 {
			t.Errorf("level %d: got %d entries", level, len(entries))
		}
	}
}

func TestUnpack_Corrupted(t *testing.T) {
	_, err := Unpack([]byte("not a tar.zst archive"))
	if err == nil {
		t.Error("expected error for corrupted archive")
	}
}
