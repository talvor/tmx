package tmx

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// LoadReader function loads tiled map in TMX format from io.Reader
// baseDir is used for loading additional tile data, current directory is used if empty
func tmxReader(source string, r io.Reader) (*Map, error) {
	d := xml.NewDecoder(r)

	baseDir := filepath.Dir(source)
	m := &Map{
		baseDir: baseDir,
		Source:  source,
	}
	if err := d.Decode(m); err != nil {
		return nil, err
	}

	sort.Slice(m.Tilesets, func(i, j int) bool { return m.Tilesets[i].FirstGID > m.Tilesets[j].FirstGID })

	err := m.decodeLayers()
	if err != nil {
		return nil, err
	}

	m.decodeTilesets()

	return m, nil
}

// LoadFile function loads tiled map in TMX format from file
func LoadFile(fileName string) (*Map, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return tmxReader(fileName, f)
}
