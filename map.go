package tmx

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"errors"
	"io"
	"path"
	"strconv"
	"strings"
)

const (
	GIDHorizontalFlip = 0x80000000
	GIDVerticalFlip   = 0x40000000
	GIDDiagonalFlip   = 0x20000000
	GIDFlip           = GIDHorizontalFlip | GIDVerticalFlip | GIDDiagonalFlip
	GIDMask           = 0x0fffffff
)

var (
	ErrUnknownEncoding       = errors.New("tmx: invalid encoding scheme")
	ErrUnknownCompression    = errors.New("tmx: invalid compression method")
	ErrInvalidDecodedDataLen = errors.New("tmx: invalid decoded data length")
	ErrInvalidPointsField    = errors.New("tmx: invalid points string")
	ErrLayerNotFound         = errors.New("tmx: layer not found")
)

type (
	GID uint32 // A tile ID. Could be used for GID or ID.
	ID  uint32
)

// All structs have their fields exported, and you'll be on the safe side as long as treat them read-only (anyone want to write 100 getters?).
type Map struct {
	baseDir      string
	Source       string
	Version      string        `xml:"title,attr"`
	Class        string        `xml:"class,attr"`
	Orientation  string        `xml:"orientation,attr"`
	Width        int           `xml:"width,attr"`
	Height       int           `xml:"height,attr"`
	TileWidth    int           `xml:"tilewidth,attr"`
	TileHeight   int           `xml:"tileheight,attr"`
	Properties   []Property    `xml:"properties>property"`
	Tilesets     []Tileset     `xml:"tileset"`
	Layers       []Layer       `xml:"layer"`
	ObjectGroups []ObjectGroup `xml:"objectgroup"`
}

func (m *Map) GetLayer(name string) (*Layer, error) {
	for i := range m.Layers {
		if m.Layers[i].Name == name {
			return &m.Layers[i], nil
		}
	}
	return nil, ErrLayerNotFound
}

func (m *Map) DecodeTileGID(gid GID) (*Tileset, GID) {
	for i := range m.Tilesets {
		ts := &m.Tilesets[i]
		if gid >= ts.FirstGID {
			return ts, gid - ts.FirstGID
		}
	}
	return nil, 0
}

type Tileset struct {
	FirstGID GID    `xml:"firstgid,attr"`
	Source   string `xml:"source,attr"`
}

type Layer struct {
	Name       string     `xml:"name,attr"`
	Width      int        `xml:"width,attr"`
	Height     int        `xml:"height,attr"`
	OffsetX    int        `xml:"offsetx,attr"`
	OffsetY    int        `xml:"offsety,attr"`
	Opacity    float32    `xml:"opacity,attr"`
	Visible    bool       `xml:"visible,attr"`
	Properties []Property `xml:"properties>property"`
	Data       *Data      `xml:"data"`
	Tiles      []GID
	Empty      bool // Set when all entries of the layer are NilTile
}

func (l *Layer) GetTilePositionFromIndex(tileIdx int, m *Map) (int, int) {
	x := tileIdx % l.Width
	y := tileIdx / l.Width
	return l.OffsetX + x*m.TileWidth, l.OffsetY + y*m.TileHeight
}

type Data struct {
	Encoding    string     `xml:"encoding,attr"`
	Compression string     `xml:"compression,attr"`
	RawData     []byte     `xml:",innerxml"`
	DataTiles   []DataTile `xml:"tile"` // Only used when layer encoding is xml
}

type ObjectGroup struct {
	Name       string     `xml:"name,attr"`
	Color      string     `xml:"color,attr"`
	Opacity    float32    `xml:"opacity,attr"`
	Visible    bool       `xml:"visible,attr"`
	Properties []Property `xml:"properties>property"`
	Objects    []Object   `xml:"object"`
}

type Object struct {
	Name       string     `xml:"name,attr"`
	Type       string     `xml:"type,attr"`
	X          float64    `xml:"x,attr"`
	Y          float64    `xml:"y,attr"`
	Width      float64    `xml:"width,attr"`
	Height     float64    `xml:"height,attr"`
	GID        int        `xml:"gid,attr"`
	Visible    bool       `xml:"visible,attr"`
	Polygons   []Polygon  `xml:"polygon"`
	PolyLines  []PolyLine `xml:"polyline"`
	Properties []Property `xml:"properties>property"`
}

type Polygon struct {
	Points string `xml:"points,attr"`
}

type PolyLine struct {
	Points string `xml:"points,attr"`
}

type Property struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

func (d *Data) decodeBase64() (data []byte, err error) {
	rawData := bytes.TrimSpace(d.RawData)
	r := bytes.NewReader(rawData)

	encr := base64.NewDecoder(base64.StdEncoding, r)

	var comr io.Reader
	switch d.Compression {
	case "gzip":
		comr, err = gzip.NewReader(encr)
		if err != nil {
			return
		}
	case "zlib":
		comr, err = zlib.NewReader(encr)
		if err != nil {
			return
		}
	case "":
		comr = encr
	default:
		err = ErrUnknownCompression
		return
	}

	return io.ReadAll(comr)
}

func (d *Data) decodeCSV() (data []GID, err error) {
	cleaner := func(r rune) rune {
		if (r >= '0' && r <= '9') || r == ',' {
			return r
		}
		return -1
	}
	rawDataClean := strings.Map(cleaner, string(d.RawData))

	str := strings.Split(string(rawDataClean), ",")

	gids := make([]GID, len(str))
	for i, s := range str {
		var d uint64
		d, err = strconv.ParseUint(s, 10, 32)
		if err != nil {
			return
		}
		gids[i] = GID(d)
	}
	return gids, err
}

func (m *Map) decodeLayerXML(l *Layer) (gids []GID, err error) {
	if len(l.Data.DataTiles) != m.Width*m.Height {
		return []GID{}, ErrInvalidDecodedDataLen
	}

	gids = make([]GID, len(l.Data.DataTiles))
	for i := range gids {
		gids[i] = l.Data.DataTiles[i].GID
	}

	return gids, nil
}

func (m *Map) decodeLayerCSV(l *Layer) ([]GID, error) {
	gids, err := l.Data.decodeCSV()
	if err != nil {
		return []GID{}, err
	}

	if len(gids) != m.Width*m.Height {
		return []GID{}, ErrInvalidDecodedDataLen
	}

	return gids, nil
}

func (m *Map) decodeLayerBase64(l *Layer) ([]GID, error) {
	dataBytes, err := l.Data.decodeBase64()
	if err != nil {
		return []GID{}, err
	}

	if len(dataBytes) != m.Width*m.Height*4 {
		return []GID{}, ErrInvalidDecodedDataLen
	}

	gids := make([]GID, m.Width*m.Height)

	j := 0
	for y := range m.Height {
		for x := range m.Width {
			gid := GID(dataBytes[j]) +
				GID(dataBytes[j+1])<<8 +
				GID(dataBytes[j+2])<<16 +
				GID(dataBytes[j+3])<<24
			j += 4

			gids[y*m.Width+x] = gid
		}
	}

	return gids, nil
}

func (m *Map) decodeLayer(l *Layer) ([]GID, error) {
	switch l.Data.Encoding {
	case "csv":
		return m.decodeLayerCSV(l)
	case "base64":
		return m.decodeLayerBase64(l)
	case "": // XML "encoding"
		return m.decodeLayerXML(l)
	}
	return []GID{}, ErrUnknownEncoding
}

func (m *Map) decodeLayers() (err error) {
	for i := range m.Layers {
		l := &m.Layers[i]
		var gids []GID
		if gids, err = m.decodeLayer(l); err != nil {
			return err
		}

		l.Tiles = gids
		l.Data = nil
	}
	return nil
}

func (m *Map) decodeTilesets() {
	for i := range m.Tilesets {
		ts := &m.Tilesets[i]
		if ts.Source == "" {
			return
		}
		ts.Source = path.Join(m.baseDir, ts.Source)
	}
}

type Point struct {
	X int
	Y int
}

type DataTile struct {
	GID GID `xml:"gid,attr"`
}

func (p *Polygon) Decode() ([]Point, error) {
	return decodePoints(p.Points)
}

func (p *PolyLine) Decode() ([]Point, error) {
	return decodePoints(p.Points)
}

func decodePoints(s string) (points []Point, err error) {
	pointStrings := strings.Split(s, " ")

	points = make([]Point, len(pointStrings))
	for i, pointString := range pointStrings {
		coordStrings := strings.Split(pointString, ",")
		if len(coordStrings) != 2 {
			return []Point{}, ErrInvalidPointsField
		}

		points[i].X, err = strconv.Atoi(coordStrings[0])
		if err != nil {
			return []Point{}, err
		}

		points[i].Y, err = strconv.Atoi(coordStrings[1])
		if err != nil {
			return []Point{}, err
		}
	}
	return
}
