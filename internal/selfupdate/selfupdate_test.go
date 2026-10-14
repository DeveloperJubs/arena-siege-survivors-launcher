package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		current, latest string
		want            bool
	}{
		{"v0.0.2", "v0.0.3", true},
		{"v0.0.9", "v0.0.10", true},
		{"v0.9.9", "v1.0.0", true},
		{"v0.0.3", "v0.0.3", false},
		{"v0.1.0", "v0.0.9", false},
		{"dev", "v0.0.3", false},
		{"v0.0.2-3-gabc1234-dirty", "v0.0.3", false},
		{"v0.0.2", "v0.0.3-rc1", false},
	} {
		if got := Newer(tc.current, tc.latest); got != tc.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
		}
	}
}

// fakeGitHub serves a latest release with the given exe, and a SHA256SUMS that matches sumOf.
func fakeGitHub(t *testing.T, exe, sumOf []byte) {
	t.Helper()
	sum := sha256.Sum256(sumOf)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/releases/latest":
			fmt.Fprintf(w, `{"tag_name":"v0.0.3","html_url":"%[1]s/release","assets":[
				{"name":"Launcher.exe","browser_download_url":"%[1]s/dl/Launcher.exe"},
				{"name":"SHA256SUMS","browser_download_url":"%[1]s/dl/SHA256SUMS"}]}`, srv.URL)
		case "/dl/Launcher.exe":
			w.Write(exe)
		case "/dl/SHA256SUMS":
			fmt.Fprintf(w, "%s  Launcher.exe\n", hex.EncodeToString(sum[:]))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	old := APIBase
	APIBase = srv.URL
	t.Cleanup(func() { APIBase = old })
}

func TestApply(t *testing.T) {
	newExe := []byte(strings.Repeat("new launcher ", 1000))
	fakeGitHub(t, newExe, newExe)
	exe := filepath.Join(t.TempDir(), "Launcher.exe")
	os.WriteFile(exe, []byte("old launcher"), 0o755)

	rel, err := Latest(context.Background(), "o/r", "Launcher.exe")
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "v0.0.3" {
		t.Errorf("Version = %q", rel.Version)
	}
	var last int64
	if err := Apply(context.Background(), rel, exe, func(done, total int64) { last = done }); err != nil {
		t.Fatal(err)
	}
	if last != int64(len(newExe)) {
		t.Errorf("progress reached %d of %d bytes", last, len(newExe))
	}
	if b, _ := os.ReadFile(exe); string(b) != string(newExe) {
		t.Error("the launcher was not replaced")
	}
	if b, _ := os.ReadFile(exe + ".old"); string(b) != "old launcher" {
		t.Errorf(".old = %q", b)
	}
	Cleanup(exe)
	if _, err := os.Stat(exe + ".old"); !os.IsNotExist(err) {
		t.Error("Cleanup left the old launcher")
	}
}

func TestApplyDamaged(t *testing.T) {
	fakeGitHub(t, []byte("tampered"), []byte("what was published"))
	exe := filepath.Join(t.TempDir(), "Launcher.exe")
	os.WriteFile(exe, []byte("old launcher"), 0o755)

	rel, err := Latest(context.Background(), "o/r", "Launcher.exe")
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(context.Background(), rel, exe, nil); err == nil || !strings.Contains(err.Error(), "damaged") {
		t.Fatalf("err = %v, want a damaged-download error", err)
	}
	if b, _ := os.ReadFile(exe); string(b) != "old launcher" {
		t.Error("a damaged download replaced the launcher")
	}
	if _, err := os.Stat(exe + ".new"); !os.IsNotExist(err) {
		t.Error("the damaged download was left behind")
	}
}

func TestLatestWithoutAssets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v0.0.3","assets":[]}`)
	}))
	defer srv.Close()
	old := APIBase
	APIBase = srv.URL
	defer func() { APIBase = old }()
	if _, err := Latest(context.Background(), "o/r", "Launcher.exe"); err == nil {
		t.Error("a release without the exe was accepted")
	}
}
