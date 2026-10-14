// Package selfupdate replaces the running launcher with the latest one published as a GitHub
// release (the files "make push" uploads: the exe and SHA256SUMS).
package selfupdate

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// APIBase is the GitHub API, replaceable in tests.
var APIBase = "https://api.github.com"

const sumsAsset = "SHA256SUMS"

// Release is a published launcher version.
type Release struct {
	Version string // "v0.0.3"
	PageURL string
	exeURL  string
	sumsURL string
	exeName string
}

// Latest returns the newest release of repo ("owner/name") that has the asset exeName.
func Latest(ctx context.Context, repo, exeName string) (*Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, APIBase+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub: %s", resp.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&body); err != nil {
		return nil, err
	}
	rel := &Release{Version: body.TagName, PageURL: body.HTMLURL, exeName: exeName}
	for _, a := range body.Assets {
		switch a.Name {
		case exeName:
			rel.exeURL = a.URL
		case sumsAsset:
			rel.sumsURL = a.URL
		}
	}
	if rel.exeURL == "" || rel.sumsURL == "" {
		return nil, fmt.Errorf("release %s has no %s and %s", body.TagName, exeName, sumsAsset)
	}
	return rel, nil
}

var versionRe = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

// IsRelease reports whether v is a release version (vX.Y.Z), not a development build.
func IsRelease(v string) bool {
	return versionRe.MatchString(v)
}

// Newer reports whether release version latest comes after current. Versions that are not
// vX.Y.Z never count as newer, nor have anything newer.
func Newer(current, latest string) bool {
	c, l := versionRe.FindStringSubmatch(current), versionRe.FindStringSubmatch(latest)
	if c == nil || l == nil {
		return false
	}
	for i := 1; i <= 3; i++ {
		a, _ := strconv.Atoi(c[i])
		b, _ := strconv.Atoi(l[i])
		if a != b {
			return b > a
		}
	}
	return false
}

// Apply downloads rel's exe, checks it against the release's SHA256SUMS and puts it in place of
// exePath. The running program carries on as the old version (moved to exePath+".old") until it
// starts the new one; see Restart and Cleanup.
func Apply(ctx context.Context, rel *Release, exePath string, progress func(done, total int64)) error {
	want, err := expectedSum(ctx, rel)
	if err != nil {
		return err
	}
	newPath, oldPath := exePath+".new", exePath+".old"
	defer os.Remove(newPath)
	got, err := download(ctx, rel.exeURL, newPath, progress)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	if got != want {
		return errors.New("the downloaded launcher is damaged (its SHA-256 does not match the release's); try again")
	}
	if st, err := os.Stat(exePath); err == nil {
		os.Chmod(newPath, st.Mode().Perm())
	}

	os.Remove(oldPath)
	if err := os.Rename(exePath, oldPath); err != nil {
		return fmt.Errorf("could not replace the launcher (is its folder writable?): %w", err)
	}
	if err := os.Rename(newPath, exePath); err != nil {
		os.Rename(oldPath, exePath)
		return fmt.Errorf("could not put the new launcher in place: %w", err)
	}
	return nil
}

// Restart starts exePath (the new launcher) with args. The caller should then quit.
func Restart(exePath string, args []string) error {
	cmd := exec.Command(exePath, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// Cleanup removes the old launcher an update left behind. That one may still be closing, so it
// tries for a few seconds.
func Cleanup(exePath string) {
	old := exePath + ".old"
	for i := 0; i < 20; i++ {
		err := os.Remove(old)
		if err == nil || errors.Is(err, os.ErrNotExist) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func expectedSum(ctx context.Context, rel *Release) (string, error) {
	resp, err := get(ctx, rel.sumsURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(io.LimitReader(resp.Body, 1<<20))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == rel.exeName {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("%s has no line for %s", sumsAsset, rel.exeName)
}

func download(ctx context.Context, url, dest string, progress func(done, total int64)) (sum string, err error) {
	resp, err := get(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	var done int64
	buf := make([]byte, 256<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return "", err
			}
			h.Write(buf[:n])
			done += int64(n)
			if progress != nil {
				progress(done, resp.ContentLength)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return "", rerr
		}
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return resp, nil
}
