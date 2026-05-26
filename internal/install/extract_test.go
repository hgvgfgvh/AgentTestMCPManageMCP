package install

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractZipAndFlatten(t *testing.T) {
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, "a.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	inner, _ := w.Create("root-dir/hello.txt")
	_, _ = inner.Write([]byte("hi"))
	_ = w.Close()
	_ = f.Close()

	dest := filepath.Join(tmp, "out")
	if err := ExtractArchive(zipPath, dest); err != nil {
		t.Fatal(err)
	}
	_ = FlattenSingleRootDir(dest)
	if _, err := os.Stat(filepath.Join(dest, "hello.txt")); err != nil {
		t.Fatalf("expected hello.txt: %v", err)
	}
}
