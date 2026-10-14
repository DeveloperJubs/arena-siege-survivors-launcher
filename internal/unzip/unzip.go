// Package unzip unpacks release zips over an install folder.
package unzip

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Options tunes Extract. The zero value unpacks everything.
type Options struct {
	// Skip leaves out files by their slash-separated path inside the folder, after the zip's
	// single top folder (if it has one) is taken off.
	Skip func(rel string) bool
	// Progress is called with the bytes unpacked so far and the total.
	Progress func(done, total int64)
}

// Extract unpacks zipPath into dest, replacing files already there. When every entry is inside
// one top folder (MyGame\...), that folder's contents go straight into dest. Names written by
// Windows tools with backslashes are handled. A failed file never leaves half a file behind.
func Extract(ctx context.Context, zipPath, dest string, opt Options) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	names := make([]string, len(zr.File))
	var total int64
	for i, f := range zr.File {
		names[i] = strings.ReplaceAll(f.Name, `\`, "/")
		total += int64(f.UncompressedSize64)
	}
	prefix := commonTopDir(names)

	var done int64
	for i, f := range zr.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		rel := strings.TrimPrefix(names[i], prefix)
		if rel == "" || strings.HasSuffix(rel, "/") || f.FileInfo().IsDir() {
			continue
		}
		if !filepath.IsLocal(filepath.FromSlash(rel)) {
			return fmt.Errorf("zip has a bad file name: %q", f.Name)
		}
		if opt.Skip != nil && opt.Skip(rel) {
			done += int64(f.UncompressedSize64)
			continue
		}
		report := func(n int64) {
			if opt.Progress != nil {
				opt.Progress(done+n, total)
			}
		}
		if err := extractFile(ctx, f, filepath.Join(dest, filepath.FromSlash(rel)), report); err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		done += int64(f.UncompressedSize64)
	}
	return nil
}

// commonTopDir returns "dir/" when every name is inside the same top-level folder, else "".
func commonTopDir(names []string) string {
	top := ""
	for _, n := range names {
		dir, _, ok := strings.Cut(n, "/")
		if !ok || dir == "" || (top != "" && dir != top) {
			return ""
		}
		top = dir
	}
	if top == "" {
		return ""
	}
	return top + "/"
}

func extractFile(ctx context.Context, f *zip.File, dest string, report func(int64)) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".unzip-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, &countingReader{ctx: ctx, r: src, report: report}); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	mode := f.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	if err := os.Chmod(tmp.Name(), mode|0o200); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}

type countingReader struct {
	ctx    context.Context
	r      io.Reader
	n      int64
	report func(int64)
}

func (c *countingReader) Read(b []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := c.r.Read(b)
	c.n += int64(n)
	c.report(c.n)
	return n, err
}
