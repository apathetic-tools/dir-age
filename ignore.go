package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const ignoreFileName = ".dir-age-ignore"

// defaultSkipDirs are directory names commonly containing metadata,
// dependencies, or build output rather than a project's actual content.
// Their timestamps (VCS bookkeeping, reinstalled packages, rebuilt
// artifacts) don't reflect when the surrounding directory was really
// created or last updated, so they're excluded by default.
var defaultSkipDirs = map[string]bool{
	// version control
	".git": true,
	".svn": true,
	".hg":  true,
	".bzr": true,

	// dependencies / packages
	"node_modules": true,
	"vendor":       true,
	".venv":        true,
	"venv":         true,
	"__pycache__":  true,
	".tox":         true,

	// build output / caches
	"build":         true,
	"dist":          true,
	"target":        true,
	".next":         true,
	".nuxt":         true,
	".cache":        true,
	".mypy_cache":   true,
	".pytest_cache": true,
}

// resolveSkipDirs picks the *base* set of directory names to skip for a scan
// rooted at root, in order of preference:
//  1. the nearest `.dir-age-ignore` found by searching root itself and then
//     its ancestors up the filesystem, so a config file above the scanned
//     path (e.g. at a project root) still applies when you point dir-age at
//     one of its subdirectories;
//  2. a `.dir-age-ignore` file next to the dir-age executable, so a user can
//     set a personal default regardless of what directory they run it from;
//  3. the built-in default list.
//
// Only one of these applies at a time — a found ignore file fully replaces
// the list rather than adding to it, keeping the override simple to reason
// about. This is only the starting point for the scan: while walking, a
// `.dir-age-ignore` found in a deeper subdirectory further overrides the
// rules for that subtree only, the same way .gitignore cascades — see
// analyze in main.go.
func resolveSkipDirs(root string) map[string]bool {
	return resolveSkipDirsFrom(root, executableDir())
}

// executableDir returns the directory containing the running executable, or
// "" if it can't be determined.
func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

// resolveSkipDirsFrom holds the precedence logic itself, taking the
// executable's directory as a plain argument so it can be exercised directly
// in tests without depending on os.Executable().
func resolveSkipDirsFrom(root, exeDir string) map[string]bool {
	if names, ok := findIgnoreUpward(root); ok {
		return names
	}

	if exeDir != "" {
		if names, ok := readIgnoreFile(filepath.Join(exeDir, ignoreFileName)); ok {
			return names
		}
	}

	return defaultSkipDirs
}

// findIgnoreUpward looks for a `.dir-age-ignore` in dir, then dir's parent,
// and so on up to the filesystem root, returning the first one found.
func findIgnoreUpward(dir string) (map[string]bool, bool) {
	current, err := filepath.Abs(dir)
	if err != nil {
		current = dir
	}
	for {
		if names, ok := readIgnoreFile(filepath.Join(current, ignoreFileName)); ok {
			return names, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil, false
		}
		current = parent
	}
}

// readIgnoreFile parses a plain-text ignore file: one directory name per
// line, blank lines and "#" comments ignored.
func readIgnoreFile(path string) (map[string]bool, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	names := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := stripComment(scanner.Text())
		if line != "" {
			names[line] = true
		}
	}
	return names, true
}

func stripComment(line string) string {
	if i := strings.IndexByte(line, '#'); i >= 0 {
		line = line[:i]
	}
	return strings.TrimSpace(line)
}
