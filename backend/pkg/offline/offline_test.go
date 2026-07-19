package offline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindImageTarFromManifest(t *testing.T) {
	dir := t.TempDir()
	images := filepath.Join(dir, "images")
	if err := os.MkdirAll(images, 0755); err != nil {
		t.Fatal(err)
	}
	tarPath := filepath.Join(images, "demo.tar")
	if err := os.WriteFile(tarPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := `version: "1"
images:
  demo/app:latest: images/demo.tar
debs: {}
drivers: {}
`
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	s := New(dir)
	got, ok := s.FindImageTar("demo/app:latest")
	if !ok || got != tarPath {
		t.Fatalf("FindImageTar()=%q ok=%v, want %q", got, ok, tarPath)
	}
}

func TestSafePathConventionFallback(t *testing.T) {
	dir := t.TempDir()
	images := filepath.Join(dir, "images")
	_ = os.MkdirAll(images, 0755)
	tarPath := filepath.Join(images, "tinylab_linux-lab.tar")
	_ = os.WriteFile(tarPath, []byte("x"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte("version: \"1\"\nimages: {}\n"), 0644)

	s := New(dir)
	got, ok := s.FindImageTar("tinylab/linux-lab")
	if !ok || got != tarPath {
		t.Fatalf("convention fallback failed: got=%q ok=%v", got, ok)
	}
}

func TestFindModelTarFromManifest(t *testing.T) {
	dir := t.TempDir()
	models := filepath.Join(dir, "models")
	if err := os.MkdirAll(models, 0755); err != nil {
		t.Fatal(err)
	}
	tarPath := filepath.Join(models, "libretranslate-en-zh.tar")
	if err := os.WriteFile(tarPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := `version: "1"
images: {}
models:
  libretranslate-en-zh: models/libretranslate-en-zh.tar
debs: {}
drivers: {}
`
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	s := New(dir)
	got, ok := s.FindModelTar(LibreTranslateModelsKey)
	if !ok || got != tarPath {
		t.Fatalf("FindModelTar()=%q ok=%v, want %q", got, ok, tarPath)
	}
	if !s.HasModels(LibreTranslateModelsKey) {
		t.Fatal("HasModels()=false")
	}
}
