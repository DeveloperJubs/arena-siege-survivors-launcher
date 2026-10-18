package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
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

// GitHubAPI is the GitHub API, replaceable in tests.
var GitHubAPI = "https://api.github.com"

// errNotFound is returned (wrapped) by get and download when the server answers 404.
var errNotFound = errors.New("not found")

// latestRe matches the path of a GitHub "latest release" download URL, naming owner and repo.
var latestRe = regexp.MustCompile(`^/([^/]+)/([^/]+)/releases/latest/download$`)

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
	if errors.Is(err, errNotFound) {
		if fb, ferr := fallbackBase(ctx, base); ferr == nil {
			body, err = get(ctx, fb+"/version.txt")
		}
	}
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

	err = download(ctx, base+"/"+updateZip, tmp, progress)
	if errors.Is(err, errNotFound) {
		if fb, ferr := fallbackBase(ctx, base); ferr == nil {
			err = download(ctx, fb+"/"+updateZip, tmp, progress)
		}
	}
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return unzip.Extract(ctx, tmp.Name(), c.Root, unzip.Options{Skip: skip})
}

// fallbackBase is for when base is a GitHub "latest release" URL whose release lacks the files
// (say it was published before they were uploaded): it returns the download URL of the newest
// published release that has both version.txt and the update zip.
func fallbackBase(ctx context.Context, base string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	m := latestRe.FindStringSubmatch(u.Path)
	if m == nil {
		return "", fmt.Errorf("%s is not a GitHub latest release", base)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GitHubAPI+"/repos/"+m[1]+"/"+m[2]+"/releases?per_page=20", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub: %s", resp.Status)
	}
	var releases []struct {
		Draft      bool `json:"draft"`
		Prerelease bool `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&releases); err != nil {
		return "", err
	}
	// Newest first, as GitHub lists them.
	for _, r := range releases {
		if r.Draft || r.Prerelease {
			continue
		}
		var zipURL string
		hasVersion := false
		for _, a := range r.Assets {
			switch a.Name {
			case updateZip:
				zipURL = a.URL
			case "version.txt":
				hasVersion = true
			}
		}
		if zipURL != "" && hasVersion {
			return strings.TrimSuffix(zipURL, "/"+updateZip), nil
		}
	}
	return "", errors.New("no release has the game's files")
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
		return nil, httpError(url, resp)
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
		return httpError(url, resp)
	}
	var r io.Reader = resp.Body
	if progress != nil {
		r = &progressReader{r: resp.Body, total: resp.ContentLength, fn: progress}
	}
	_, err = io.Copy(w, r)
	return err
}

func httpError(url string, resp *http.Response) error {
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%s: %w", url, errNotFound)
	}
	return fmt.Errorf("%s: %s", url, resp.Status)
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
