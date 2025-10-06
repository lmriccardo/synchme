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
	info, err := os.Stat(path)
	if err != nil {
		ERROR("Error: ", err)
		return nil
	}

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

// MkdirAll creates all the folders (including the non-existing parent
// and calls a callback on each created folder)
func MkdirAll(path string, mode os.FileMode, callback func(string) error) error {
	paths := []string{}
	curr_path := path

	// First collects all the paths up to the first existing one
	for {
		_, err := os.Stat(curr_path)

		// If the current folder does not exists
		if os.IsNotExist(err) {
			paths = append(paths, curr_path)
			curr_path = filepath.Dir(curr_path)
			continue
		}

		break
	}

	// Create the folder and all subfolders
	if err := os.MkdirAll(path, mode); err != nil {
		return err
	}

	for idx := len(paths) - 1; idx > -1; idx-- {
		if err := callback(paths[idx]); err != nil {
			return err
		}
	}

	return nil
}

// RemoveAll removes all the sub folders and files contained in the input path
// and calls a callback for each removed element
func RemoveAll(path string, callback func(string) error) error {
	subpaths := WalkDir(path)

	for idx := len(subpaths) - 1; idx > -1; idx-- {
		if err := os.Remove(subpaths[idx]); err != nil {
			return err
		}

		if err := callback(subpaths[idx]); err != nil {
			return err
		}
	}

	return nil
}

// IsFileOpen checks if a file is open/locked by another process.
// It automatically picks the correct implementation based on OS.
func IsFileOpen(path string) bool {
	return isFileOpen(path)
}

// Exist checks if a file exists in the current machine
func Exist(path string) bool {
	_, err := os.Stat(path)
	return os.IsExist(err)
}
