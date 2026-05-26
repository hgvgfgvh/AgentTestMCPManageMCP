package install

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractArchive 将 zip 或 .tar.gz/.tgz 解压到 destDir。
func ExtractArchive(archivePath, destDir string) error {
	lower := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(archivePath, destDir)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractTarGz(archivePath, destDir)
	default:
		// 尝试 zip 魔数
		if isZipFile(archivePath) {
			return extractZip(archivePath, destDir)
		}
		return fmt.Errorf("unsupported archive %q", filepath.Base(archivePath))
	}
}

func isZipFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 4)
	n, _ := f.Read(buf)
	return n >= 2 && buf[0] == 'P' && buf[1] == 'K'
}

func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if err := extractZipEntry(f, destDir); err != nil {
			return err
		}
	}
	return nil
}

func extractZipEntry(f *zip.File, destDir string) error {
	name := filepath.Clean(filepath.FromSlash(f.Name))
	if strings.HasPrefix(name, "..") {
		return fmt.Errorf("zip slip: %s", f.Name)
	}
	target := filepath.Join(destDir, name)
	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode()&0o777|0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, io.LimitReader(rc, 128<<20))
	_ = out.Close()
	return err
}

func extractTarGz(path, destDir string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(filepath.FromSlash(hdr.Name))
		if strings.HasPrefix(name, "..") {
			return fmt.Errorf("tar slip: %s", hdr.Name)
		}
		target := filepath.Join(destDir, name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode)&0o777|0o600)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, io.LimitReader(tr, 128<<20))
			_ = out.Close()
			if err != nil {
				return err
			}
		}
	}
}

// FlattenSingleRootDir 若解压后仅有一层根目录，将其内容上移一层（GitHub archive 常见）。
func FlattenSingleRootDir(dir string) error {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(ents) != 1 || !ents[0].IsDir() {
		return nil
	}
	root := filepath.Join(dir, ents[0].Name())
	sub, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, e := range sub {
		old := filepath.Join(root, e.Name())
		newPath := filepath.Join(dir, e.Name())
		if err := os.Rename(old, newPath); err != nil {
			return err
		}
	}
	return os.Remove(root)
}
