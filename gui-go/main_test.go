package main

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestPrintVersion(t *testing.T) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	printVersion()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Pulse-Vault")) || !bytes.Contains(buf.Bytes(), []byte("version:")) {
		t.Fatalf("version banner missing pieces: %q", out)
	}
}
