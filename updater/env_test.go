package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- env file editing ------------------------------------------------------

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSetEnvFileKey_UpdateExisting(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	writeFile(t, p, `# header comment
FOO=before
BAR=42
PROCESSGIT_VERSION=0.1.0
BAZ=keepme
`)
	prev, had, err := SetEnvFileKey(p, "PROCESSGIT_VERSION", "0.1.2")
	if err != nil {
		t.Fatal(err)
	}
	if !had || prev != "0.1.0" {
		t.Fatalf("expected hadKey=true prev=0.1.0; got had=%v prev=%q", had, prev)
	}
	got := readFile(t, p)
	if !strings.Contains(got, "PROCESSGIT_VERSION=0.1.2\n") {
		t.Fatalf("update missing in result:\n%s", got)
	}
	// other keys preserved
	for _, want := range []string{"FOO=before", "BAR=42", "BAZ=keepme", "# header comment"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q preserved; got:\n%s", want, got)
		}
	}
}

func TestSetEnvFileKey_AppendsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	writeFile(t, p, "FOO=existing\n")
	prev, had, err := SetEnvFileKey(p, "PROCESSGIT_VERSION", "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if had {
		t.Fatalf("expected hadKey=false; got true")
	}
	if prev != "" {
		t.Fatalf("expected empty previous value, got %q", prev)
	}
	got := readFile(t, p)
	if !strings.Contains(got, "FOO=existing") {
		t.Error("FOO should be preserved")
	}
	if !strings.Contains(got, "PROCESSGIT_VERSION=0.1.0") {
		t.Errorf("appended value missing:\n%s", got)
	}
}

func TestSetEnvFileKey_FileDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	prev, had, err := SetEnvFileKey(p, "FOO", "bar")
	if err != nil {
		t.Fatal(err)
	}
	if had || prev != "" {
		t.Fatalf("unexpected prev/had on fresh file: prev=%q had=%v", prev, had)
	}
	got := readFile(t, p)
	if !strings.Contains(got, "FOO=bar") {
		t.Errorf("expected new file to contain FOO=bar; got:\n%s", got)
	}
}

func TestSetEnvFileKey_QuotedValues(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	writeFile(t, p, `PROCESSGIT_VERSION="0.1.0"`+"\n")
	prev, _, err := SetEnvFileKey(p, "PROCESSGIT_VERSION", "0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if prev != "0.1.0" {
		t.Fatalf("expected unquoted previous value 0.1.0; got %q", prev)
	}
}

func TestSetEnvFileKey_DuplicatesCommented(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	writeFile(t, p, `PROCESSGIT_VERSION=first
FOO=bar
PROCESSGIT_VERSION=second
`)
	prev, had, err := SetEnvFileKey(p, "PROCESSGIT_VERSION", "third")
	if err != nil {
		t.Fatal(err)
	}
	if !had || prev != "first" {
		t.Fatalf("expected hadKey=true prev=first; got had=%v prev=%q", had, prev)
	}
	got := readFile(t, p)
	if !strings.Contains(got, "PROCESSGIT_VERSION=third") {
		t.Error("first occurrence not updated")
	}
	if !strings.Contains(got, "# PROCESSGIT_VERSION=second") {
		t.Errorf("duplicate not commented out:\n%s", got)
	}
}

func TestSetEnvFileKey_EmptyKeyRejected(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	_, _, err := SetEnvFileKey(p, "", "value")
	if err == nil {
		t.Fatal("expected error on empty key")
	}
}

func TestGetEnvFileKey(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	writeFile(t, p, `# comment
FOO=bar
EMPTY=
QUOTED="hello world"
`)
	tests := []struct {
		key     string
		wantVal string
		wantOK  bool
	}{
		{"FOO", "bar", true},
		{"EMPTY", "", true},
		{"QUOTED", "hello world", true},
		{"MISSING", "", false},
	}
	for _, tt := range tests {
		v, ok, err := GetEnvFileKey(p, tt.key)
		if err != nil {
			t.Fatalf("%s: %v", tt.key, err)
		}
		if v != tt.wantVal || ok != tt.wantOK {
			t.Errorf("%s: got val=%q ok=%v; want %q %v", tt.key, v, ok, tt.wantVal, tt.wantOK)
		}
	}
}

// --- imageVersion helper ---------------------------------------------------

func TestImageVersion(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"ghcr.io/foo/bar:0.1.2", "0.1.2"},
		{"ghcr.io/foo/bar:latest", "latest"},
		{"ghcr.io/foo/bar:0.1.0-rc1", "0.1.0-rc1"},
		{"ghcr.io/foo/bar@sha256:abcdef", ""}, // digest form has no tag
		{"registry:5000/foo/bar:0.1", "0.1"},  // port in registry hostname
		{"0.1.2", "0.1.2"},                    // bare version
		{"bar:0.1.2", "0.1.2"},
		{"bar", "bar"}, // no colon at all — treat whole as version
	}
	for _, tt := range tests {
		if got := imageVersion(tt.ref); got != tt.want {
			t.Errorf("imageVersion(%q) = %q; want %q", tt.ref, got, tt.want)
		}
	}
}

// --- lastLines ------------------------------------------------------------

func TestLastLines(t *testing.T) {
	in := "a\nb\nc\nd\ne\n"
	if got := lastLines(in, 2); got != "d\ne" {
		t.Errorf("lastLines(_, 2) = %q; want %q", got, "d\ne")
	}
	if got := lastLines(in, 10); got != "a\nb\nc\nd\ne" {
		t.Errorf("lastLines(_, 10) = %q", got)
	}
	if got := lastLines("", 5); got != "" {
		t.Errorf("lastLines(empty, 5) = %q", got)
	}
	if got := lastLines("only one", 1); got != "only one" {
		t.Errorf("lastLines(single, 1) = %q", got)
	}
}
