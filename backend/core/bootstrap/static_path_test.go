package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeWWWPath_KeepsInsideRoot(t *testing.T) {
	www := t.TempDir()
	absWWW, err := filepath.Abs(www)
	if err != nil {
		t.Fatal(err)
	}

	cases := []string{
		"/../../etc/passwd",
		"/../db/rde.db",
		"/foo/../../../etc/passwd",
		"//etc/passwd",
	}
	for _, req := range cases {
		got, ok := safeWWWPath(www, req)
		if !ok {
			// rejection is fine
			continue
		}
		if got != absWWW && !strings.HasPrefix(got, absWWW+string(os.PathSeparator)) {
			t.Fatalf("path escaped www root: req=%q got=%q www=%q", req, got, absWWW)
		}
	}
}

func TestSafeWWWPath_AllowsNested(t *testing.T) {
	www := t.TempDir()
	nested := filepath.Join(www, "_app", "immutable", "x.js")
	if err := os.MkdirAll(filepath.Dir(nested), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}

	got, ok := safeWWWPath(www, "/_app/immutable/x.js")
	if !ok {
		t.Fatal("expected nested path allowed")
	}
	absNested, _ := filepath.Abs(nested)
	if got != absNested {
		t.Fatalf("got %q want %q", got, absNested)
	}
}

func TestIsAllowedOrigin(t *testing.T) {
	if !isAllowedOrigin("http://192.168.1.10:3080", "192.168.1.10:3080") {
		t.Fatal("same host should allow")
	}
	if isAllowedOrigin("http://evil.example", "192.168.1.10:3080") {
		t.Fatal("cross origin should deny")
	}
}
