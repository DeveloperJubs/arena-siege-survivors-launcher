package stonetavern

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/danotchuuy/arena-siege-survivors-launcher/internal/unzip"
)

// stateFile, in an install folder, records which edition and build it holds.
const stateFile = "stonetavern-client.json"

// Installed is what an install folder holds.
type Installed struct {
	Edition string `json:"edition"`
	Build   string `json:"build"` // "" while an install or update is unfinished
	SHA256  string `json:"sha256,omitempty"`
	URL     string `json:"url"`
}

// Stage names a step of Install, for progress reports.
type Stage string

const (
	Downloading Stage = "Downloading"
	Verifying   Stage = "Checking"
	Unpacking   Stage = "Unpacking"
)

// Progress reports how far a stage of Install has got, in bytes; total is -1 if unknown.
type Progress func(stage Stage, done, total int64)

// FolderName is the folder an edition installs into: "Stonetavern-Classic".
func FolderName(ed Edition) string {
	name := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return -1
	}, ed.Name)
	return "Stonetavern-" + name
}

// DataDir is the WoW "Data" folder inside an install folder.
func DataDir(dir string) string {
	return filepath.Join(dir, "Data")
}

// ReadInstalled returns what was installed into dir by Install, or an error if Install did not
// make it.
func ReadInstalled(dir string) (*Installed, error) {
	b, err := os.ReadFile(filepath.Join(dir, stateFile))
	if err != nil {
		return nil, err
	}
	var in Installed
	if err := json.Unmarshal(b, &in); err != nil {
		return nil, fmt.Errorf("%s: %w", stateFile, err)
	}
	return &in, nil
}

// IsNewerThan reports whether ed should replace what is installed.
func (ed Edition) IsNewerThan(in *Installed) bool {
	return in.Build == "" || ed.Build != in.Build || (ed.SHA256 != "" && in.SHA256 != "" && ed.SHA256 != in.SHA256)
}

// Install downloads ed's zip next to dir, checks it against the published hash and unpacks it
// into dir, over an older build if one is there. An interrupted download carries on where it
// stopped the next time. The zip is deleted once unpacked.
func Install(ctx context.Context, ed Edition, dir string, progress Progress) error {
	if progress == nil {
		progress = func(Stage, int64, int64) {}
	}
	u, err := url.Parse(ed.URL)
	if err != nil || u.Scheme != "https" {
		return fmt.Errorf("bad download address %q", ed.URL)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	zipPath := filepath.Join(filepath.Dir(dir), path.Base(u.Path)+".part")

	if err := download(ctx, ed.URL, zipPath, func(done, total int64) { progress(Downloading, done, total) }); err != nil {
		return fmt.Errorf("download: %w", err)
	}
	if ed.SHA256 != "" {
		sum, err := hashFile(ctx, zipPath, func(done, total int64) { progress(Verifying, done, total) })
		if err != nil {
			return err
		}
		if sum != ed.SHA256 {
			os.Remove(zipPath)
			return errors.New("the download is damaged (its SHA-256 does not match the published one); try again")
		}
	}

	// Until unpacking is done, the folder is marked unfinished, so an update is offered again.
	state := Installed{Edition: ed.Name, SHA256: ed.SHA256, URL: ed.URL}
	if err := writeState(dir, state); err != nil {
		return err
	}
	err = unzip.Extract(ctx, zipPath, dir, unzip.Options{
		Skip:     func(rel string) bool { return rel == stateFile },
		Progress: func(done, total int64) { progress(Unpacking, done, total) },
	})
	if err != nil {
		return fmt.Errorf("unpack: %w", err)
	}
	state.Build = ed.Build
	if err := writeState(dir, state); err != nil {
		return err
	}
	os.Remove(zipPath)
	return nil
}

func writeState(dir string, in Installed) error {
	b, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, stateFile), append(b, '\n'), 0o644)
}

// download fetches url into dest, carrying on from what dest already holds when the server
// allows it.
func download(ctx context.Context, url, dest string, progress func(done, total int64)) error {
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	have, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	from := ""
	if have > 0 {
		from = strconv.FormatInt(have, 10)
	}
	resp, err := get(ctx, url, from)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	total := int64(-1)
	switch {
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && have > 0:
		return nil // dest already holds all of it
	case resp.StatusCode == http.StatusPartialContent && resumesAt(resp.Header.Get("Content-Range"), have):
		if resp.ContentLength >= 0 {
			total = have + resp.ContentLength
		}
	case resp.StatusCode == http.StatusOK:
		if err := f.Truncate(0); err != nil {
			return err
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		have, total = 0, resp.ContentLength
	default:
		return fmt.Errorf("%s: %s", url, resp.Status)
	}

	done := have
	buf := make([]byte, 1<<20)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			done += int64(n)
			progress(done, total)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	if total >= 0 && done != total {
		return fmt.Errorf("download stopped at %d of %d bytes", done, total)
	}
	return f.Close()
}

// resumesAt reports whether a Content-Range header ("bytes 100-199/200") starts at offset.
func resumesAt(contentRange string, offset int64) bool {
	return strings.HasPrefix(contentRange, "bytes "+strconv.FormatInt(offset, 10)+"-")
}

func hashFile(ctx context.Context, name string, progress func(done, total int64)) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	h := sha256.New()
	buf := make([]byte, 1<<20)
	var done int64
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := f.Read(buf)
		h.Write(buf[:n])
		done += int64(n)
		progress(done, st.Size())
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
