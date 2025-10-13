package utils

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// MultiWalkDir recursively walks the directory trees rooted at each of the
// provided 'paths' and aggregates all discovered file paths.
//
// It iterates through the input paths, calls the external function WalkDir
// on each one, and appends the resulting list of file paths to a final slice.
// Errors returned by WalkDir (represented by a nil return) are effectively
// ignored, and only valid results are aggregated.
func MultiWalkDir(paths ...string) (result []string) {
	for _, path := range paths {
		if r := WalkDir(path); r != nil {
			result = append(result, r...)
		}
	}

	return
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

// MkdirNoErr creates the directory specified by 'path' if it does not already exist.
//
// If the directory creation fails, it either logs a fatal error and exits the program
// or logs a non-fatal error, based on the 'fatal_on_err' flag. This function does
// create any necessary parent directories.
func MkdirNoErr(path string, perm os.FileMode, fatal_on_err bool) {
	if !Exist(path) {
		if err := os.MkdirAll(path, perm); err != nil {
			if fatal_on_err {
				FATAL("Unable to create ", path, ": ", err)
			} else {
				ERROR("Unable to create ", path, ": ", err)
			}
		}
	}
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
	return err == nil
}

// IsDir checks if the input path is a directory or not
func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// LongestCommonPrefixPath computes the longest common prefix path among a
// slice of file or directory paths.
//
// The function splits all paths by the operating system's file separator
// (e.g., '/' or '\') and iteratively reduces the prefix path components until
// a common sequence is found across all input paths.
func LongestCommonPrefixPath(paths []string) string {
	// If the input slice of paths is empty returns an empty string
	if len(paths) == 0 {
		return ""
	}

	// Helper split function
	split := func(p string) []string {
		return strings.Split(p, string(filepath.Separator))
	}

	// Otherwise compute the longest common prefix path among all input paths.
	// It is constructed iteratively starting from all components of the first
	// path in the slice, and then reducing the number of parts when there is
	// a mismatch with a component of another path
	parts := split(paths[0])
	for _, path := range paths[1:] {
		path_parts := split(path)
		min_size := min(len(parts), len(path_parts))
		m_idx := 0

		// Iterate until we find an index such that the current
		// part of the prefix does not match a part of the curr path
		for m_idx < min_size && parts[m_idx] == path_parts[m_idx] {
			m_idx++
		}

		// Take the subset of parts until the match index
		parts = parts[:m_idx]

		// If there are no more parts then we can break, essentially
		// there is no a common prefix among input paths
		if len(parts) == 0 {
			break
		}
	}

	// Re-join all parts into a prefix string.
	return strings.Join(parts, string(filepath.Separator))
}

// RelativizePaths takes a slice of absolute file and directory paths and transforms them
// into a slice of context-aware relative paths.
//
// This is a two-step process:
//  1. Grouping: All paths are first reduced by their global Longest Common Prefix (LCP)
//     to determine a set of top-level directories (groups).
//  2. Relativization: For each resulting group, the LCP is re-calculated, and all paths
//     within that group are made relative to that group's LCP. This ensures that the
//     returned paths start from the most meaningful, unique, top-level folder component.
//
// The process handles directories (paths ending with a separator) and files, resulting
// in an output that provides the minimal necessary path components for unique identification
// within the context of the input set.
func RelativizePaths(paths []string) []string {
	// If the input slice of paths is empty returns nil
	if len(paths) == 0 {
		return nil
	}

	// Sort all the paths so that the top level folder is always the first
	sort.Strings(paths)

	// Normalize paths removing all unnecessary elements
	for p_idx, path := range paths {
		paths[p_idx] = filepath.Clean(path)
	}

	// The relativization comes with two necessary steps to complete.
	// In the first step it needs to groups all folders/files that have
	// the same parent folder. To do this, it is important to remove the
	// most top-level folder prefix so that it remains with relatives
	// paths with respect to the top-level folders it would like to group by.
	top_level_prefix := LongestCommonPrefixPath(paths)
	separator := string(filepath.Separator)

	// Helper trim function to remove the prefix and the separator
	trim := func(s, prefix string) string {
		return strings.TrimPrefix(strings.TrimPrefix(s, prefix), separator)
	}

	path_groups := make(map[string][]string)
	for _, path := range paths {
		// Remove the top level folder and the leading separator (if any)
		relative := trim(path, top_level_prefix)
		if relative == "" {
			top_level_prefix = filepath.Dir(top_level_prefix)
			relative = trim(path, top_level_prefix)
		}

		// Take the new top-level folder as the group name
		parts := strings.SplitN(relative, separator, 2)
		top := parts[0]
		path_groups[top] = append(path_groups[top], relative)
	}

	// In the second step, for each group recomputes the longest common
	// prefix path and compute the relative path wrt to that prefix.
	relatives := []string{}
	for top, group := range path_groups {
		group_prefix := LongestCommonPrefixPath(group)

		// For each path in the group compute relativization with respect
		// to the new longest common prefix path.
		for _, path := range group {
			// Remove the top level folder and the leading separator (if any)
			relative := trim(path, group_prefix)

			if relative == "" {
				// If the relative path is empty, it means that the LCP path
				// is the folder itself, therefore replace top with the
				// last-level folder in the current path
				top = filepath.Base(strings.TrimPrefix(path, separator))
				relatives = append(relatives, top)
			} else {
				relatives = append(relatives, filepath.Join(top, relative))
			}
		}
	}

	return relatives
}

// ListFolder returns the list of subfolders and file (depth 1)
func ListFolder(root string) []string {
	// First check that the input path is itself a folder and exists
	if !IsDir(root) {
		return nil
	}

	// If the check is successul then we can list for content
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	dir_content := make([]string, 0, len(entries))
	for _, entry := range entries {
		dir_content = append(dir_content, filepath.Join(root, entry.Name()))
	}

	return dir_content
}

// Copy recursively copies a file or directory from src to dstRoot.
// If src is a file, it's copied directly into dstRoot.
// If src is a directory, its entire tree is replicated under dstRoot.
func Copy(src_path, dst_root string) error {
	// Check that the source path exists
	info, err := os.Stat(src_path)
	if err != nil {
		return fmt.Errorf("source path does not exist: %w", err)
	}

	// Define helper function to copy a single file efficiently
	copyFile := func(srcFile, dstFile string, perm fs.FileMode) error {
		in, err := os.Open(srcFile)
		if err != nil {
			return fmt.Errorf("failed to open source file: %w", err)
		}

		defer ErrorHandler(in.Close)

		flags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
		out, err := os.OpenFile(dstFile, flags, perm)
		if err != nil {
			return fmt.Errorf("failed to create destination file: %w", err)
		}

		defer ErrorHandler(out.Close)

		// Stream data directly without loading entire file into memory
		if _, err = io.Copy(out, in); err != nil {
			return fmt.Errorf("failed to copy file contents: %w", err)
		}

		// Flush data to disk
		if err = out.Sync(); err != nil {
			return fmt.Errorf("failed to sync destination file: %w", err)
		}

		return nil
	}

	// If the path is a file then we can easily create a new file with the same
	// name and copy its content into the destination
	if !info.IsDir() {
		dstFile := filepath.Join(dst_root, filepath.Base(src_path))
		return copyFile(src_path, dstFile, info.Mode())
	}

	// Otherwise, it is a folder. In this case we need to copy the entire tree
	// into the destination folder. First, we need to create the input folder
	dstRootFolder := filepath.Join(dst_root, filepath.Base(src_path))
	if err := os.MkdirAll(dstRootFolder, info.Mode()); err != nil {
		return fmt.Errorf("failed to create destination root: %w", err)
	}

	// Walk the source directory tree
	if err = filepath.WalkDir(src_path,
		func(subpath string, d fs.DirEntry, walkErr error) error {
			// If the input error is different from nil then we
			// cannot continue diving into the folder tree and just
			// returns the error
			if walkErr != nil {
				return walkErr
			}

			// If the current path is the root folder just continue
			if subpath == src_path {
				return nil
			}

			// Compute destination path by replacing the root part
			dstPath := strings.Replace(subpath, src_path, dstRootFolder, 1)

			// Get file info for permissions
			info, err := d.Info()
			if err != nil {
				return err
			}

			if d.IsDir() {
				// Create directory with same permissions
				if err := os.MkdirAll(dstPath, info.Mode()); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", dstPath, err)
				}
			} else {
				// If it is a file we can directly use the helper function
				// since its parent folder has already been created in a previous
				// step of the 'recursion'
				if err := copyFile(subpath, dstPath, info.Mode()); err != nil {
					return err
				}
			}

			return nil
		},
	); err != nil {
		return fmt.Errorf("error while walking directory: %w", err)
	}

	return nil
}
