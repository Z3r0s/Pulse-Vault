package main

import (
	"strings"
	"testing"
)

func TestHumanSize(t *testing.T) {
	if humanSize(0) != "0 B" {
		t.Fatalf("0: %q", humanSize(0))
	}
	if humanSize(512) != "512 B" {
		t.Fatalf("512: %q", humanSize(512))
	}
	got := humanSize(1536)
	if !strings.Contains(got, "KB") {
		t.Fatalf("1536: %q", got)
	}
	got = humanSize(2 * 1024 * 1024)
	if !strings.Contains(got, "MB") {
		t.Fatalf("2MiB: %q", got)
	}
}

func TestStripANSIAndVisLen(t *testing.T) {
	raw := "\x1b[38;5;50mhello\x1b[0m"
	if stripANSI(raw) != "hello" {
		t.Fatalf("strip=%q", stripANSI(raw))
	}
	if visLen(raw) != 5 {
		t.Fatalf("visLen=%d", visLen(raw))
	}
}

func TestBoxContainsTitleAndBody(t *testing.T) {
	s := box("inventory", []string{"secret.bin", "(empty vault)"}, 40)
	if !strings.Contains(s, "inventory") {
		t.Fatalf("missing title: %s", s)
	}
	if !strings.Contains(s, "secret.bin") {
		t.Fatalf("missing body: %s", s)
	}
	if !strings.Contains(s, "╭") || !strings.Contains(s, "╰") {
		t.Fatalf("missing box drawing: %s", s)
	}
}

func TestTruncate(t *testing.T) {
	if truncate("ab", 5) != "ab" {
		t.Fatal(truncate("ab", 5))
	}
	got := truncate("abcdefghij", 5)
	if visLen(got) != 5 || !strings.HasSuffix(got, "...") {
		t.Fatalf("got %q", got)
	}
}
