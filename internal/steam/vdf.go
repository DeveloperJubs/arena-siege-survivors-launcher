// Package steam adds Arena Siege Survivors to a player's Steam library as a non-Steam game, so
// Steam shows their friends what they are playing. Steam keeps those games in a binary KeyValues
// file, userdata/<account>/config/shortcuts.vdf; this package reads it, adds the game once, and
// writes it back with every other entry exactly as it was.
package steam

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

// The binary KeyValues value types. Steam writes maps, strings and 32-bit ints in shortcuts.vdf;
// the others are read and written back untouched.
const (
	typeMap    byte = 0x00
	typeString byte = 0x01
	typeInt    byte = 0x02
	typeFloat  byte = 0x03
	typePtr    byte = 0x04
	typeColor  byte = 0x06
	typeUint64 byte = 0x07
	typeEnd    byte = 0x08
)

// node is one KeyValues entry: a map (Children) or a value (Str, or Raw for the number types).
type node struct {
	Type     byte
	Key      string
	Str      string
	Raw      []byte
	Children []*node
}

func (n *node) child(key string) *node {
	for _, c := range n.Children {
		if strings.EqualFold(c.Key, key) {
			return c
		}
	}
	return nil
}

func (n *node) str(key string) string {
	if c := n.child(key); c != nil && c.Type == typeString {
		return c.Str
	}
	return ""
}

func stringNode(key, value string) *node { return &node{Type: typeString, Key: key, Str: value} }

func intNode(key string, value uint32) *node {
	raw := make([]byte, 4)
	binary.LittleEndian.PutUint32(raw, value)
	return &node{Type: typeInt, Key: key, Raw: raw}
}

// parse reads the entries of a map up to its end marker (or the end of the file, at the top).
func parse(r *bytes.Reader, top bool) ([]*node, error) {
	var out []*node
	for {
		t, err := r.ReadByte()
		if err == io.EOF && top {
			return out, nil
		}
		if err != nil {
			return nil, errors.New("shortcuts.vdf ends early")
		}
		if t == typeEnd {
			return out, nil
		}
		key, err := cstring(r)
		if err != nil {
			return nil, err
		}
		n := &node{Type: t, Key: key}
		switch t {
		case typeMap:
			if n.Children, err = parse(r, false); err != nil {
				return nil, err
			}
		case typeString:
			if n.Str, err = cstring(r); err != nil {
				return nil, err
			}
		case typeInt, typeFloat, typePtr, typeColor:
			n.Raw = make([]byte, 4)
		case typeUint64:
			n.Raw = make([]byte, 8)
		default:
			return nil, fmt.Errorf("shortcuts.vdf has a value of unknown type %d", t)
		}
		if n.Raw != nil {
			if _, err := io.ReadFull(r, n.Raw); err != nil {
				return nil, errors.New("shortcuts.vdf ends early")
			}
		}
		out = append(out, n)
	}
}

func cstring(r *bytes.Reader) (string, error) {
	var b []byte
	for {
		c, err := r.ReadByte()
		if err != nil {
			return "", errors.New("shortcuts.vdf ends early")
		}
		if c == 0 {
			return string(b), nil
		}
		b = append(b, c)
	}
}

// write writes entries, then the end marker that closes their map.
func write(w *bytes.Buffer, nodes []*node) {
	for _, n := range nodes {
		w.WriteByte(n.Type)
		w.WriteString(n.Key)
		w.WriteByte(0)
		switch n.Type {
		case typeMap:
			write(w, n.Children)
		case typeString:
			w.WriteString(n.Str)
			w.WriteByte(0)
		default:
			w.Write(n.Raw)
		}
	}
	w.WriteByte(typeEnd)
}

// decode reads a whole shortcuts.vdf; an empty file is an empty one.
func decode(data []byte) ([]*node, error) {
	return parse(bytes.NewReader(data), true)
}

// encode writes a whole shortcuts.vdf, as Steam does: the top level closes with an end marker too.
func encode(nodes []*node) []byte {
	var w bytes.Buffer
	write(&w, nodes)
	return w.Bytes()
}
