package dataset

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

func normalizeLocalDataset(d *LocalDataset) {
	if d == nil || len(d.FileEntries) == 0 {
		return
	}

	entries := make([]LocalDatasetFile, 0, len(d.FileEntries))
	seen := make(map[string]struct{}, len(d.FileEntries))
	for _, entry := range d.FileEntries {
		relPath := cleanLocalDatasetPath(entry.Path)
		if relPath == "" || relPath == "manifest.json" {
			continue
		}
		if _, ok := seen[relPath]; ok {
			continue
		}
		seen[relPath] = struct{}{}
		entry.Path = relPath
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	d.FileEntries = entries

	files := localDatasetFilePaths(entries)
	if len(files) > 0 {
		d.Files = files
	}
}

func EnsureLocalDatasetFiles(datasetDir string, d *LocalDataset) (bool, error) {
	if d == nil {
		return false, nil
	}

	normalizeLocalDataset(d)

	known := make(map[string]LocalDatasetFile, len(d.FileEntries))
	for _, entry := range d.FileEntries {
		known[entry.Path] = entry
	}

	entries := make([]LocalDatasetFile, 0, len(d.FileEntries))
	seen := make(map[string]struct{}, len(d.FileEntries))
	changed := false

	err := filepath.WalkDir(datasetDir, func(fullPath string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fullPath == datasetDir {
			return nil
		}

		relPath, err := filepath.Rel(datasetDir, fullPath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		if dirEntry.Type()&os.ModeSymlink != 0 {
			changed = true
			if dirEntry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if dirEntry.IsDir() {
			return nil
		}
		if relPath == "manifest.json" {
			return nil
		}

		info, err := dirEntry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			changed = true
			return nil
		}

		entry, ok := known[relPath]
		if !ok {
			entry = LocalDatasetFile{Path: relPath}
			changed = true
		}

		if entry.Path != relPath {
			entry.Path = relPath
			changed = true
		}
		if entry.Size != info.Size() {
			entry.Size = info.Size()
			entry.SHA256 = ""
			changed = true
		}
		if entry.SHA256 == "" {
			sum, err := fileSHA256(fullPath)
			if err != nil {
				return err
			}
			entry.SHA256 = sum
			changed = true
		}

		entries = append(entries, entry)
		seen[relPath] = struct{}{}
		return nil
	})
	if err != nil {
		return false, err
	}

	for relPath := range known {
		if _, ok := seen[relPath]; !ok {
			changed = true
			break
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	files := localDatasetFilePaths(entries)
	if !reflect.DeepEqual(d.FileEntries, entries) {
		d.FileEntries = entries
		changed = true
	}
	if !reflect.DeepEqual(d.Files, files) {
		d.Files = files
		changed = true
	}

	return changed, nil
}

func cleanLocalDatasetPath(relPath string) string {
	trimmed := strings.TrimSpace(filepath.ToSlash(relPath))
	if trimmed == "" {
		return ""
	}
	cleaned := path.Clean(trimmed)
	if cleaned == "." || cleaned == "/" || strings.HasPrefix(cleaned, "../") {
		return ""
	}
	return strings.TrimPrefix(cleaned, "./")
}

func localDatasetFilePaths(entries []LocalDatasetFile) []string {
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Path == "" {
			continue
		}
		files = append(files, entry.Path)
	}
	return files
}

func fileSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}
