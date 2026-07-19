package instance

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitPersistsAndReuses(t *testing.T) {
	dir := t.TempDir()
	id1, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	if id1 == "" {
		t.Fatal("empty id")
	}
	path := filepath.Join(dir, FileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	// reset cache and reload
	mu.Lock()
	cached = ""
	mu.Unlock()
	id2, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatalf("want same id, got %s vs %s", id1, id2)
	}
}
