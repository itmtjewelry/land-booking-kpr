package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type JSONFile struct {
	Meta  map[string]any            `json:"meta"`
	Items map[string]json.RawMessage `json:"items"`
}

type LoadResult struct {
	Dir        string
	Loaded     map[string]JSONFile // key = filename (e.g. users.json)
	LoadedList []string            // ordered
}

// LoadCore loads core JSON files from STORAGE_DIR and validates schema shape A:
// { "meta": {..}, "items": {..} }
// Strict mode: any error returns error; caller should stop service startup.
func LoadCore(storageDir string) (*LoadResult, error) {
	if storageDir == "" {
		return nil, fmt.Errorf("STORAGE_DIR is required")
	}

	info, err := os.Stat(storageDir)
	if err != nil {
		return nil, fmt.Errorf("STORAGE_DIR not accessible: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("STORAGE_DIR is not a directory: %s", storageDir)
	}

	coreFiles := []string{
		"users.json",
		"sites.json",
		"subsites.json",
		"zones.json",
		"bookings.json",
		"domains.json",
	}

	res := &LoadResult{
		Dir:        storageDir,
		Loaded:     make(map[string]JSONFile, len(coreFiles)),
		LoadedList: make([]string, 0, len(coreFiles)),
	}

	for _, name := range coreFiles {
		full := filepath.Join(storageDir, name)
		jf, err := loadOne(full)
		if err != nil {
			return nil, fmt.Errorf("load %s failed: %w", name, err)
		}
		res.Loaded[name] = *jf
		res.LoadedList = append(res.LoadedList, name)
	}

	// Support file is optional; ignore if missing
	_ = tryLoadOptional(filepath.Join(storageDir, "support", "tickets.json"))

	return res, nil
}

func loadOne(path string) (*JSONFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var jf JSONFile
	if err := json.Unmarshal(b, &jf); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}

	// Schema guard (shape A)
	if jf.Meta == nil {
		return nil, fmt.Errorf("missing meta object")
	}
	if jf.Items == nil {
		return nil, fmt.Errorf("missing items object")
	}

	return &jf, nil
}

func tryLoadOptional(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		return nil // optional: ignore missing/unreadable
	}
	_, err = loadOne(path)
	return err
}
