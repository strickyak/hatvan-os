package vm

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var listLineRegex = regexp.MustCompile(`^([0-9A-Fa-f]{4})\s+([0-9A-Fa-f]{2,})?\s*(?:\(([^)]+)\):([0-9]+))?\s*(.*)$`)

// Listing represents parsed assembly listing information.
type Listing struct {
	LinesByAddr map[uint16]SourceLine
}

// NewListing returns an initialized Listing.
func NewListing() *Listing {
	return &Listing{
		LinesByAddr: make(map[uint16]SourceLine),
	}
}

// LoadListing parses an lwasm assembly listing from a reader.
func LoadListing(r io.Reader) (*Listing, error) {
	l := NewListing()
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		m := listLineRegex.FindStringSubmatch(line)
		if m != nil {
			addr64, err := strconv.ParseUint(m[1], 16, 16)
			if err != nil {
				continue
			}
			addr := uint16(addr64)
			var lineNum uint16
			if m[4] != "" {
				ln, _ := strconv.ParseUint(m[4], 10, 16)
				lineNum = uint16(ln)
			}
			srcText := strings.TrimRight(m[5], " \t\r\n")
			if srcText == "" && len(line) > 56 {
				srcText = strings.TrimSpace(line[56:])
			}
			l.LinesByAddr[addr] = SourceLine{
				LineNum: lineNum,
				Text:    srcText,
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading listing: %w", err)
	}
	return l, nil
}

// LoadListingFile parses an lwasm assembly listing from disk.
func LoadListingFile(path string) (*Listing, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadListing(f)
}
