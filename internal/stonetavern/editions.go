// Package stonetavern downloads and updates the WoW 1.12.1 clients published at
// https://stonetavern.app/clients. Arena Siege Survivors reads its game files from a client's Data folder.
package stonetavern

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// ClientsURL is the page listing the clients.
const ClientsURL = "https://stonetavern.app/clients"

const userAgent = "ArenaSiegeSurvivorsLauncher (+https://github.com/danotchuuy/arena-siege-survivors-launcher)"

var httpClient = http.DefaultClient

// Edition is one client build offered on the clients page.
type Edition struct {
	Name        string // short name: "Classic", "Enhanced", "Cinematic"
	Title       string // as the page shows it: "Stonetavern · Classic"
	Tagline     string
	Recommended bool
	Build       string // "1.8"
	Released    string // "2026-09-23"
	Size        string // "4.96 GB"
	SHA256      string // full lowercase hex hash of the zip, or "" when the page has none
	URL         string // the zip
}

// FetchEditions reads the clients on offer from the clients page.
func FetchEditions(ctx context.Context) ([]Edition, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := get(ctx, ClientsURL, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return parseEditions(io.LimitReader(resp.Body, 16<<20))
}

// Find returns the edition called name, or nil.
func Find(editions []Edition, name string) *Edition {
	for i := range editions {
		if strings.EqualFold(editions[i].Name, name) {
			return &editions[i]
		}
	}
	return nil
}

var (
	fullHashRe  = regexp.MustCompile(`\b[0-9a-f]{64}\b`)
	shortHashRe = regexp.MustCompile(`^([0-9a-f]{4,})\s*…\s*([0-9a-f]{4,})$`)
)

// parseEditions reads the page's comparison table ("How the three 1.12.1 editions compare"):
// one column per edition, one row per fact. The table shortens the hashes ("0ebb9b5f … 79fc");
// the full ones are elsewhere on the page and are matched up by their ends.
func parseEditions(r io.Reader) ([]Edition, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	table := find(doc, func(n *html.Node) bool { return n.Data == "table" && hasClass(n, "edition-ledger") })
	if table == nil {
		return nil, errors.New("the clients page has no editions table (has the page changed?)")
	}

	var eds []Edition
	if thead := find(table, isElem("thead")); thead != nil {
		for _, th := range findAll(thead, func(n *html.Node) bool { return n.Data == "th" && hasClass(n, "edition-ledger-col") }) {
			ed := Edition{}
			if h := find(th, isElem("h2")); h != nil {
				ed.Title = text(h)
			}
			if p := find(th, func(n *html.Node) bool { return hasClass(n, "edition-ledger-tagline") }); p != nil {
				ed.Tagline = text(p)
			}
			ed.Recommended = find(th, func(n *html.Node) bool { return hasClass(n, "edition-ledger-badge") }) != nil
			ed.Name = ed.Title
			if _, after, ok := strings.Cut(ed.Title, "·"); ok {
				ed.Name = strings.TrimSpace(after)
			}
			eds = append(eds, ed)
		}
	}
	if len(eds) == 0 {
		return nil, errors.New("the clients page lists no editions")
	}

	fullHashes := fullHashRe.FindAllString(text(doc), -1)
	tbody := find(table, isElem("tbody"))
	if tbody == nil {
		return nil, errors.New("the editions table is empty")
	}
	for _, tr := range findAll(tbody, isElem("tr")) {
		th := find(tr, isElem("th"))
		if th == nil {
			continue
		}
		cells := findAll(tr, isElem("td"))
		if len(cells) != len(eds) {
			continue
		}
		for i, td := range cells {
			ed := &eds[i]
			switch text(th) {
			case "Build":
				ed.Build = text(td)
			case "Released":
				ed.Released = text(td)
			case "Size":
				ed.Size = text(td)
			case "SHA-256":
				ed.SHA256 = matchHash(text(td), fullHashes)
			case "Download":
				if a := find(td, isElem("a")); a != nil {
					ed.URL = attr(a, "href")
				}
			}
		}
	}

	for _, ed := range eds {
		if ed.Name == "" || ed.Build == "" || !strings.HasPrefix(ed.URL, "https://") {
			return nil, fmt.Errorf("could not read the %q edition from the clients page", ed.Title)
		}
	}
	return eds, nil
}

// matchHash turns a shortened hash ("0ebb9b5f … 79fc") into the full one it stands for.
func matchHash(short string, full []string) string {
	short = strings.ToLower(strings.TrimSpace(short))
	if fullHashRe.MatchString(short) && len(short) == 64 {
		return short
	}
	m := shortHashRe.FindStringSubmatch(short)
	if m == nil {
		return ""
	}
	for _, h := range full {
		if strings.HasPrefix(h, m[1]) && strings.HasSuffix(h, m[2]) {
			return h
		}
	}
	return ""
}

func isElem(tag string) func(*html.Node) bool {
	return func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == tag }
}

func find(n *html.Node, match func(*html.Node) bool) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && match(c) {
			return c
		}
		if f := find(c, match); f != nil {
			return f
		}
	}
	return nil
}

func findAll(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && match(c) {
			out = append(out, c)
			continue
		}
		out = append(out, findAll(c, match)...)
	}
	return out
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, class string) bool {
	return n.Type == html.ElementNode && strings.Contains(" "+attr(n, "class")+" ", " "+class+" ")
}

// text is n's visible text with runs of white space made single spaces. Scripts count too, which
// is where some of the page's data sits.
func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func get(ctx context.Context, url, rangeFrom string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	if rangeFrom != "" {
		req.Header.Set("Range", "bytes="+rangeFrom+"-")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusPartialContent, http.StatusRequestedRangeNotSatisfiable:
	default:
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return resp, nil
}
