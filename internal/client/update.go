package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/danotchuuy/arena-siege-survivors-launcher/internal/unzip"
)

const updateZip = "ArenaSiegeSurvivorsClient-update.zip"

// DefaultReleases is where Arena Siege Survivors releases are published, as a new client's
// update.txt names it.
var DefaultReleases = "https://github.com/DeveloperJubs/ArenaSiegeSurvivors-Releases/releases/latest/download"

// oldReleases is where the game was published while it was called Rogue Arena. Folders from then
// still name it in update.txt; they are updated from DefaultReleases instead.
const oldReleases = "https://github.com/unicornjubs1-create/RogueArena-Releases/releases/latest/download"

// keep lists the player's own files, which an update never overwrites (slash-separated).
var keep = []string{"login.txt", "wow-data.txt", "game/data", "Play Arena Siege Survivors.bat"}

// Progress is called while downloading; total is -1 when the size is unknown.
type Progress func(done, total int64)

// updateBase is the release URL from update.txt, or "" when updates are off.
func (c *Client) updateBase() string {
	s, _ := readTrimmed(c.path("update.txt"))
	s = strings.TrimRight(s, "/")
	if strings.EqualFold(s, oldReleases) {
		return DefaultReleases
	}
	return s
}

// CheckUpdate reports whether update.txt's release site has a newer version.txt than this folder.
func (c *Client) CheckUpdate(ctx context.Context) (bool, error) {
	base := c.updateBase()
	if base == "" {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	body, err := get(ctx, base+"/version.txt")
	if err != nil {
		return false, err
	}
	defer body.Close()
	b, err := io.ReadAll(io.LimitReader(body, 1024))
	if err != nil {
		return false, err
	}
	return parseVersion(string(b)) > c.LocalVersion(), nil
}

// Update downloads the latest release zip and unpacks it over this folder, keeping the player's
// files (see keep).
func (c *Client) Update(ctx context.Context, progress Progress) error {
	base := c.updateBase()
	if base == "" {
		return errors.New("no update.txt")
	}
	return c.fetchAndUnpack(ctx, base, kept, progress)
}

// Install downloads the latest release from DefaultReleases (or this folder's update.txt, if it
// has one) and unpacks it into Root, which is made if needed. Files the player already has there
// (see keep) are left alone.
func (c *Client) Install(ctx context.Context, progress Progress) error {
	if err := os.MkdirAll(c.Root, 0o755); err != nil {
		return err
	}
	base := c.updateBase()
	if base == "" {
		base = DefaultReleases
	}
	skip := func(rel string) bool {
		if !kept(rel) {
			return false
		}
		_, err := os.Stat(c.path(filepath.FromSlash(rel)))
		return err == nil
	}
	if err := c.fetchAndUnpack(ctx, base, skip, progress); err != nil {
		return err
	}
	return c.Check()
}

// fetchAndUnpack downloads the release zip from base and unpacks it into Root, leaving out the
// files skip names.
func (c *Client) fetchAndUnpack(ctx context.Context, base string, skip func(rel string) bool, progress Progress) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	tmp, err := os.CreateTemp("", "arena-siege-survivors-update-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if err := download(ctx, base+"/"+updateZip, tmp, progress); err != nil {
		return fmt.Errorf("download: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return unzip.Extract(ctx, tmp.Name(), c.Root, unzip.Options{Skip: skip})
}

func get(ctx context.Context, url string) (io.ReadCloser, error) {
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
	return resp.Body, nil
}

func download(ctx context.Context, url string, w io.Writer, progress Progress) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	var r io.Reader = resp.Body
	if progress != nil {
		r = &progressReader{r: resp.Body, total: resp.ContentLength, fn: progress}
	}
	_, err = io.Copy(w, r)
	return err
}

type progressReader struct {
	r     io.Reader
	done  int64
	total int64
	fn    Progress
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	p.fn(p.done, p.total)
	return n, err
}

// installZip unpacks zipPath over root, keeping the player's files. The release zip holds one
// folder (ArenaSiegeSurvivorsClient\...), whose contents go straight into root.
func installZip(zipPath, root string) error {
	return unzip.Extract(context.Background(), zipPath, root, unzip.Options{Skip: kept})
}

func kept(rel string) bool {
	rel = path.Clean(rel)
	for _, k := range keep {
		if strings.EqualFold(rel, k) || strings.HasPrefix(strings.ToLower(rel), strings.ToLower(k)+"/") {
			return true
		}
	}
	return false
}
