package tmx

import (
	"errors"
	"os"
	"path/filepath"
)

var (
	ErrMapManagerNotLoaded = errors.New("tsx: map manager not loaded")
	ErrMapNotFound         = errors.New("tsx: map not found")
)

type MapManager struct {
	baseDir  string
	Maps     map[string]*Map
	IsLoaded bool
}

func (mm *MapManager) GetMapByName(name string) (*Map, error) {
	if !mm.IsLoaded {
		return nil, ErrMapManagerNotLoaded
	}

	if m, ok := mm.Maps[name]; ok {
		return m, nil
	}

	return nil, ErrMapNotFound
}

func NewMapManager(baseDir string) *MapManager {
	mm := &MapManager{
		baseDir:  baseDir,
		Maps:     make(map[string]*Map),
		IsLoaded: false,
	}

	LoadMaps(mm)

	return mm
}

func LoadMaps(mm *MapManager) {
	tsxFiles, err := findTMXFiles(mm.baseDir)
	if err != nil {
		return
	}

	for _, tsxFile := range tsxFiles {
		t, err := LoadFile(tsxFile)
		if err != nil {
			panic(err)
		}

		mm.Maps[t.Class] = t
	}

	mm.IsLoaded = true
}

func findTMXFiles(dir string) ([]string, error) {
	var tmxFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".tmx" {
			tmxFiles = append(tmxFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return tmxFiles, nil
}
