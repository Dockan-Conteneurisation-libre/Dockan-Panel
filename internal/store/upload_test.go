package store

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type memFile struct{ *bytes.Reader }

func (memFile) Close() error { return nil }

func zipOf(t *testing.T, add func(w *zip.Writer)) memFile {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	add(w)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return memFile{bytes.NewReader(buf.Bytes())}
}

func put(t *testing.T, w *zip.Writer, name, body string, mode os.FileMode) {
	t.Helper()
	h := &zip.FileHeader{Name: name, Method: zip.Deflate}
	h.SetMode(mode)
	f, err := w.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
}

func TestCleanSitePath(t *testing.T) {
	good := map[string]string{"index.html": "index.html", "/css/./a.css": "css/a.css", `img\b.png`: "img/b.png"}
	for in, want := range good {
		if got, err := CleanSitePath(in); err != nil || got != want {
			t.Errorf("CleanSitePath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "../x", "a/../../x", "./", "a\x00b"} {
		if _, err := CleanSitePath(in); err == nil {
			t.Errorf("CleanSitePath(%q) accepted", in)
		}
	}
}

func openRoot(t *testing.T, dir string) *os.Root {
	t.Helper()
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

func TestExtractZipStaysInsideTheSite(t *testing.T) {
	public := t.TempDir()
	root := openRoot(t, public)
	ok := zipOf(t, func(w *zip.Writer) {
		put(t, w, "index.html", "<h1>ok</h1>", 0o644)
		put(t, w, "css/site.css", "body{}", 0o644)
	})
	n, err := ExtractZip(root, ok, ok.Size())
	if err != nil || n != 2 {
		t.Fatalf("valid zip: %d files, %v", n, err)
	}
	if b, _ := os.ReadFile(filepath.Join(public, "css/site.css")); string(b) != "body{}" {
		t.Error("css/site.css not written")
	}

	evil := zipOf(t, func(w *zip.Writer) { put(t, w, "../escape.html", "x", 0o644) })
	if _, err := ExtractZip(root, evil, evil.Size()); err == nil {
		t.Error("zip with .. accepted")
	}
	link := zipOf(t, func(w *zip.Writer) { put(t, w, "link", "/etc/passwd", os.ModeSymlink|0o777) })
	if _, err := ExtractZip(root, link, link.Size()); err == nil || !strings.Contains(err.Error(), "symbolic") {
		t.Errorf("zip with a symlink: %v", err)
	}
}

func TestWriteRefusesALinkedFolder(t *testing.T) {
	public := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(public, "out")); err != nil {
		t.Fatal(err)
	}
	z := zipOf(t, func(w *zip.Writer) { put(t, w, "out/pwned.html", "x", 0o644) })
	if _, err := ExtractZip(openRoot(t, public), z, z.Size()); err == nil {
		t.Error("write through a linked folder accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "pwned.html")); err == nil {
		t.Error("a file was written outside the website folder")
	}
}

func TestOpenSiteRefusesALinkedPublicFolder(t *testing.T) {
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "dockan.yml"), []byte("name: site\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(target, "public")); err != nil {
		t.Fatal(err)
	}
	if root, _, err := OpenSite(target); err == nil {
		root.Close()
		t.Fatal("a linked public folder was accepted")
	}
}

func TestEmptyDirStaysInside(t *testing.T) {
	public, outside := t.TempDir(), t.TempDir()
	keep := filepath.Join(outside, "keep.txt")
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(public, "link")); err != nil {
		t.Fatal(err)
	}
	if err := EmptyDir(openRoot(t, public)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("EmptyDir removed a file outside the site")
	}
}
