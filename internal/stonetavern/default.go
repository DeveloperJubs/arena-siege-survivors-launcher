package stonetavern

import "fmt"

// DefaultEdition keeps the one-click path on the plain 1.12.1 data files.
// Never silently choose a different edition or accept a missing checksum.
func DefaultEdition(editions []Edition) (*Edition, error) {
	ed := Find(editions, "Classic")
	if ed == nil || !fullHashRe.MatchString(ed.SHA256) || len(ed.SHA256) != 64 {
		return nil, fmt.Errorf("verified Classic 1.12.1 download is unavailable")
	}
	return ed, nil
}
