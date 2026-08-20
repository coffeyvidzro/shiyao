package main

import (
	"bytes"
	"testing"
)

func TestLimitedBufferTruncatesWithoutShortWrite(t *testing.T) {
	var buf limitedBuffer
	buf.limit = 4

	input := []byte("abcdefgh")
	n, err := buf.Write(input)
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if n != len(input) {
		t.Fatalf("Write consumed %d bytes, want %d", n, len(input))
	}
	if !bytes.Equal(buf.buf, []byte("abcd")) {
		t.Fatalf("buffer = %q, want %q", buf.buf, "abcd")
	}
	if !buf.truncated {
		t.Fatal("expected buffer to be marked truncated")
	}
}

func TestLimitedBufferConsumesWritesAfterTruncation(t *testing.T) {
	var buf limitedBuffer
	buf.limit = 2

	if n, err := buf.Write([]byte("abc")); err != nil || n != 3 {
		t.Fatalf("first Write = (%d, %v), want (3, nil)", n, err)
	}
	if n, err := buf.Write([]byte("def")); err != nil || n != 3 {
		t.Fatalf("second Write = (%d, %v), want (3, nil)", n, err)
	}
	if got := buf.String(); got != "ab" {
		t.Fatalf("buffer = %q, want %q", got, "ab")
	}
}
