package store

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxSiteFiles = 20000
	maxSiteBytes = 2 << 30
)

// CleanSitePath turns an uploaded path into a safe relative path.
func CleanSitePath(p string) (string, error) {
	p = strings.TrimLeft(strings.ReplaceAll(strings.TrimSpace(p), `\`, "/"), "/")
	if p == "" || strings.ContainsRune(p, 0) {
		return "", errors.New("invalid uploaded file path")
	}
	var parts []string
	for _, part := range strings.Split(p, "/") {
		part = strings.TrimSpace(part)
		switch part {
		case "", ".":
			continue
		case "..":
			return "", errors.New("uploaded paths cannot contain ..")
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return "", errors.New("invalid uploaded file path")
	}
	return strings.Join(parts, "/"), nil
}

// OpenSite opens target/public as an os.Root: every later write is
// confined to it by the kernel, links included. A public folder that is a
// symbolic link is refused.
func OpenSite(target string) (*os.Root, string, error) {
	if !fileExists(filepath.Join(target, "dockan.yml")) {
		return nil, "", errors.New("install Static Site before uploading website files")
	}
	base, err := os.OpenRoot(target)
	if err != nil {
		return nil, "", fmt.Errorf("open the app folder: %w", err)
	}
	defer base.Close()
	if err := base.Mkdir("public", 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, "", fmt.Errorf("public folder: %w", err)
	}
	if st, err := base.Lstat("public"); err != nil || !st.IsDir() {
		return nil, "", errors.New("the public folder must be a real folder, not a link")
	}
	root, err := base.OpenRoot("public")
	if err != nil {
		return nil, "", fmt.Errorf("open the public folder: %w", err)
	}
	return root, filepath.Join(target, "public"), nil
}

// EmptyDir removes the content of the site.
func EmptyDir(root *os.Root) error {
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	entries, err := dir.ReadDir(-1)
	dir.Close()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := root.RemoveAll(e.Name()); err != nil {
			return err
		}
	}
	return nil
}

// writeSiteFile writes rel inside the site; the root refuses any path that
// leaves it, through ".." or a link.
func writeSiteFile(root *os.Root, rel string, src io.Reader, budget *int64) error {
	if dir := filepath.Dir(rel); dir != "." {
		if err := root.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("upload folder %s: %w", dir, err)
		}
	}
	if err := root.Remove(rel); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("replace %s: %w", rel, err)
	}
	f, err := root.OpenFile(rel, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("write %s: %w", rel, err)
	}
	n, err := io.Copy(f, io.LimitReader(src, *budget+1))
	*budget -= n
	closeErr := f.Close()
	if err != nil {
		return fmt.Errorf("write %s: %w", rel, err)
	}
	if *budget < 0 {
		return errors.New("the upload is too large")
	}
	return closeErr
}

// ExtractZip copies a website archive into public.
func ExtractZip(public *os.Root, file multipart.File, size int64) (int, error) {
	zr, err := zip.NewReader(file, size)
	if err != nil {
		return 0, errors.New("unable to open the zip file")
	}
	if len(zr.File) > maxSiteFiles {
		return 0, errors.New("the zip contains too many files")
	}
	budget := int64(maxSiteBytes)
	count := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 || !f.Mode().IsRegular() {
			return count, errors.New("the zip cannot contain symbolic links or special files")
		}
		rel, err := CleanSitePath(f.Name)
		if err != nil {
			return count, err
		}
		rc, err := f.Open()
		if err != nil {
			return count, fmt.Errorf("read %s: %w", f.Name, err)
		}
		err = writeSiteFile(public, rel, rc, &budget)
		rc.Close()
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// SaveFiles copies uploaded files, keeping their relative folders.
func SaveFiles(public *os.Root, files []*multipart.FileHeader) (int, error) {
	if len(files) > maxSiteFiles {
		return 0, errors.New("too many files")
	}
	budget := int64(maxSiteBytes)
	count := 0
	for _, fh := range files {
		rel, err := CleanSitePath(fh.Filename)
		if err != nil {
			return count, err
		}
		src, err := fh.Open()
		if err != nil {
			return count, fmt.Errorf("read %s: %w", fh.Filename, err)
		}
		err = writeSiteFile(public, rel, src, &budget)
		src.Close()
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}
