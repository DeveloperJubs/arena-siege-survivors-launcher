package stonetavern

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseEditions(t *testing.T) {
	f, err := os.Open("testdata/clients.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	eds, err := parseEditions(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(eds) != 3 {
		t.Fatalf("got %d editions, want 3: %+v", len(eds), eds)
	}
	classic := Find(eds, "classic")
	if classic == nil {
		t.Fatalf("no Classic edition in %+v", eds)
	}
	want := Edition{
		Name:     "Classic",
		Title:    "Stonetavern · Classic",
		Tagline:  "The plain client. Engine fixes only, nothing that touches combat.",
		Build:    "1.8",
		Released: "2026-09-23",
		Size:     "4.96 GB",
		SHA256:   "0ebb9b5f0386d547e0715fdae57955743dd06e991b507ca5a7729d14af0479fc",
		URL:      "https://downloads.stonetavern.app/clients/classic-1.12.1/all/1.8/Stonetavern-Classic-1.12.1-v1.8.zip",
	}
	if *classic != want {
		t.Errorf("Classic =\n%+v\nwant\n%+v", *classic, want)
	}
	if e := Find(eds, "Enhanced"); e == nil || !e.Recommended || e.SHA256 == "" {
		t.Errorf("Enhanced = %+v, want it recommended with a hash", e)
	}
}

func TestParseEditionsChangedPage(t *testing.T) {
	if _, err := parseEditions(strings.NewReader("<html><body><p>Coming soon</p></body></html>")); err == nil {
		t.Error("a page without the table parsed")
	}
}

// clientZip is a small stand-in for a client zip: one top folder with Data\ inside.
func clientZip(t *testing.T, version string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{
		"Stonetavern-Classic-1.12.1-v" + version + "/Data/dbc.MPQ":   strings.Repeat("d", 50000),
		"Stonetavern-Classic-1.12.1-v" + version + "/Data/patch.MPQ": "p",
		"Stonetavern-Classic-1.12.1-v" + version + "/VERSION":        version,
	} {
		w, _ := zw.Create(name)
		w.Write([]byte(body))
	}
	zw.Close()
	return buf.Bytes()
}

func serve(t *testing.T, files map[string][]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, r.URL.Path, time.Time{}, bytes.NewReader(b))
	}))
	t.Cleanup(srv.Close)
	old := httpClient
	httpClient = srv.Client()
	t.Cleanup(func() { httpClient = old })
	return srv
}

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestInstallAndUpdate(t *testing.T) {
	v1, v2 := clientZip(t, "1.8"), clientZip(t, "1.9")
	srv := serve(t, map[string][]byte{"/c-1.8.zip": v1, "/c-1.9.zip": v2})
	dir := filepath.Join(t.TempDir(), "Stonetavern-Classic")

	ed := Edition{Name: "Classic", Build: "1.8", SHA256: sum(v1), URL: srv.URL + "/c-1.8.zip"}
	// Half of it was downloaded before: Install carries on from there.
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "c-1.8.zip.part"), v1[:len(v1)/2], 0o644); err != nil {
		t.Fatal(err)
	}
	var firstReport int64 = -1
	err := Install(context.Background(), ed, dir, func(stage Stage, done, total int64) {
		if stage == Downloading && firstReport < 0 {
			firstReport = done
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if firstReport <= int64(len(v1)/2) {
		t.Errorf("download started over (first report at %d bytes)", firstReport)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(b) != "1.8" {
		t.Errorf("VERSION = %q", b)
	}
	if _, err := os.Stat(filepath.Join(DataDir(dir), "dbc.MPQ")); err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "c-1.8.zip.part")); !os.IsNotExist(err) {
		t.Error("the zip was left behind")
	}
	in, err := ReadInstalled(dir)
	if err != nil || in.Edition != "Classic" || in.Build != "1.8" {
		t.Fatalf("ReadInstalled = %+v, %v", in, err)
	}
	if ed.IsNewerThan(in) {
		t.Error("the same build counts as newer")
	}

	next := Edition{Name: "Classic", Build: "1.9", SHA256: sum(v2), URL: srv.URL + "/c-1.9.zip"}
	if !next.IsNewerThan(in) {
		t.Fatal("build 1.9 does not count as newer")
	}
	if err := Install(context.Background(), next, dir, nil); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(b) != "1.9" {
		t.Errorf("after the update VERSION = %q", b)
	}
	if in, _ := ReadInstalled(dir); in == nil || in.Build != "1.9" {
		t.Errorf("after the update ReadInstalled = %+v", in)
	}
}

func TestInstallBadHash(t *testing.T) {
	z := clientZip(t, "1.8")
	srv := serve(t, map[string][]byte{"/c.zip": z})
	dir := filepath.Join(t.TempDir(), "Stonetavern-Classic")
	ed := Edition{Name: "Classic", Build: "1.8", SHA256: strings.Repeat("0", 64), URL: srv.URL + "/c.zip"}
	if err := Install(context.Background(), ed, dir, nil); err == nil || !strings.Contains(err.Error(), "damaged") {
		t.Fatalf("err = %v, want a damaged-download error", err)
	}
	if _, err := os.Stat(filepath.Join(DataDir(dir), "dbc.MPQ")); !os.IsNotExist(err) {
		t.Error("a damaged download was unpacked")
	}
	if _, err := ReadInstalled(dir); err == nil {
		t.Error("a damaged download was marked installed")
	}
}

func TestFolderName(t *testing.T) {
	if got := FolderName(Edition{Name: "Cinematic"}); got != "Stonetavern-Cinematic" {
		t.Errorf("FolderName = %q", got)
	}
	if got := FolderName(Edition{Name: `..\x y`}); got != "Stonetavern-xy" {
		t.Errorf("FolderName = %q", got)
	}
}
