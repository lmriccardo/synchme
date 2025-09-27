package utils

import (
	"io/fs"
	"os"
	"path/filepath"
)

// ReadFileContent reads the content of the input file
func ReadFileContent(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		WARN("Impossible reading ", path)
		return ""
	}

	return string(content)
}

// WalkDir returns a list of all subfolder and subfiles recursively
func WalkDir(path string) []string {
	result := []string{path} // Initialize the return list

	// If the input path is just a file, returns only the file
	info, _ := os.Stat(path)
	if !info.IsDir() {
		return []string{path}
	}

	if err := filepath.WalkDir(path,
		func(subPath string, d fs.DirEntry, err error) error {
			if path != subPath {
				result = append(result, subPath)
			}

			return nil
		},
	); err != nil {
		return nil
	}

	return result
}
