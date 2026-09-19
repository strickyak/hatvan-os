package gep9

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var listLineRegex = regexp.MustCompile(`^([0-9A-Fa-f]{4})\s+([0-9A-Fa-f]{2,})?\s*(?:\(([^)]+)\):([0-9]+))?\s*(.*)$`)

// Listing represents parsed assembly listing information.
type Listing struct {
	Path        string
	ModuleName  string
	LinesByAddr map[uint16]SourceLine
}

// NewListing returns an initialized Listing.
func NewListing(path string) *Listing {
	base := strings.ToLower(filepath.Base(path))
	modName := strings.TrimSuffix(base, ".list")
	modName = strings.TrimSuffix(modName, ".lst")
	modName = strings.TrimSuffix(modName, ".listing")
	if strings.HasPrefix(modName, "kernel") {
		modName = "kernel"
	} else if strings.HasPrefix(modName, "tkt9sim") {
		modName = "tk"
	} else if strings.HasPrefix(modName, "ioman") {
		modName = "ioman"
	} else if strings.HasPrefix(modName, "go_") {
		modName = "go"
	}

	return &Listing{
		Path:        path,
		ModuleName:  modName,
		LinesByAddr: make(map[uint16]SourceLine),
	}
}

// LoadListing parses an lwasm assembly listing from a reader.
func LoadListing(r io.Reader, path string) (*Listing, error) {
	l := NewListing(path)
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
	return LoadListing(f, path)
}

// OffsetCopy returns a new Listing with all line addresses shifted by baseAddr.
func (l *Listing) OffsetCopy(baseAddr uint16) *Listing {
	nl := &Listing{
		Path:        l.Path,
		ModuleName:  l.ModuleName,
		LinesByAddr: make(map[uint16]SourceLine, len(l.LinesByAddr)),
	}
	for off, line := range l.LinesByAddr {
		nl.LinesByAddr[baseAddr+off] = line
	}
	return nl
}
