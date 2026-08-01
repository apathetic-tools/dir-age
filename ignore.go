package main

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const ignoreFileName = ".dir-age-ignore"

// defaultSkipNames are directory/file names commonly containing metadata,
// dependencies, or build output rather than a project's actual content.
// Their timestamps (VCS bookkeeping, reinstalled packages, rebuilt
// artifacts) don't reflect when the surrounding directory was really
// created or last updated, so they're excluded by default.
var defaultSkipNames = []string{
	// version control
	".git", ".svn", ".hg", ".bzr",

	// dependencies / packages
	"node_modules", "vendor", ".venv", "venv", "__pycache__", ".tox", "Pods",

	// build output / caches
	"build", "dist", "target", ".next", ".nuxt", ".cache", ".mypy_cache", ".pytest_cache",
	"CMakeFiles", ".deps", ".libs", "cmake-build-*", "GeneratedFiles", "DerivedData",

	// infra / tooling caches
	".terraform", ".gradle",
}

var defaultSkipPatterns = parseLines(defaultSkipNames)

// pattern is a single compiled line from a `.dir-age-ignore` file, following
// gitignore syntax:
//   - "name" or "*.log" (no "/", other than an optional trailing one) matches
//     that name/glob at any depth, like an implicit leading "**/".
//   - "/name" or "dir/sub" (a "/" anywhere but the end) anchors the pattern
//     to the directory containing the ignore file - it only matches starting
//     from there, not at every depth.
//   - a trailing "/" (e.g. "build/") matches directories only, never files.
//   - "**" matches zero or more path segments.
//   - "*" and "?" match within a single path segment (never across "/"),
//     and "[...]" character classes are supported, same as shell globs.
//   - a leading "!" negates the pattern, re-including anything a previous
//     pattern excluded.
type pattern struct {
	negate   bool
	dirOnly  bool
	segments []string
}

// parseLine parses one line of a `.dir-age-ignore` file into a pattern.
// Blank lines and comment lines (starting with "#") are reported via ok=false.
func parseLine(line string) (pattern, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return pattern{}, false
	}
	if line[0] == '#' {
		return pattern{}, false
	}

	negate := false
	if strings.HasPrefix(line, "!") {
		negate = true
		line = line[1:]
	}
	if strings.HasPrefix(line, `\!`) || strings.HasPrefix(line, `\#`) {
		line = line[1:]
	}
	if line == "" {
		return pattern{}, false
	}

	dirOnly := false
	if strings.HasSuffix(line, "/") {
		dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}
	if line == "" {
		return pattern{}, false
	}

	anchored := strings.Contains(line, "/")
	line = strings.TrimPrefix(line, "/")

	segments := strings.Split(line, "/")
	if !anchored {
		segments = append([]string{"**"}, segments...)
	}

	return pattern{negate: negate, dirOnly: dirOnly, segments: segments}, true
}

// parseLines parses a slice of raw lines (as if they were the contents of an
// ignore file, one entry per element) into patterns, skipping any that don't
// produce a real pattern (blank/comment).
func parseLines(lines []string) []pattern {
	patterns := make([]pattern, 0, len(lines))
	for _, line := range lines {
		if p, ok := parseLine(line); ok {
			patterns = append(patterns, p)
		}
	}
	return patterns
}

// matchAny reports whether relPath (slash-separated, relative to the
// directory the patterns are anchored to) is ignored by patterns. Patterns
// are evaluated in order and the last one that matches wins, so a later "!"
// pattern can re-include something an earlier pattern excluded - the same
// precedence rule gitignore uses.
func matchAny(patterns []pattern, relPath string, isDir bool) bool {
	relPath = filepath.ToSlash(relPath)
	segs := strings.Split(relPath, "/")

	matched := false
	for _, p := range patterns {
		if p.dirOnly && !isDir {
			continue
		}
		if matchSegments(p.segments, segs) {
			matched = !p.negate
		}
	}
	return matched
}

// matchSegments matches a pattern's path segments (where "**" stands for
// zero or more segments and each other segment is a shell glob) against a
// candidate path's segments, requiring the whole path to be consumed.
func matchSegments(pat, segs []string) bool {
	if len(pat) == 0 {
		return len(segs) == 0
	}
	if pat[0] == "**" {
		if matchSegments(pat[1:], segs) {
			return true
		}
		if len(segs) == 0 {
			return false
		}
		return matchSegments(pat, segs[1:])
	}
	if len(segs) == 0 {
		return false
	}
	ok, err := path.Match(pat[0], segs[0])
	if err != nil || !ok {
		return false
	}
	return matchSegments(pat[1:], segs[1:])
}

// ignoreRules is a set of ignore patterns together with the directory they
// are anchored to, i.e. the directory the containing `.dir-age-ignore` file
// (or, for the built-in defaults, the scan root) lives in. Anchored patterns
// like "/build" or "sub/dir" are matched relative to this directory.
type ignoreRules struct {
	anchor   string
	patterns []pattern
}

// resolveSkipDirs picks the base ignoreRules to use for a scan rooted at
// root, in order of preference:
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
func resolveSkipDirs(root string) ignoreRules {
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
func resolveSkipDirsFrom(root, exeDir string) ignoreRules {
	if rules, ok := findIgnoreUpward(root); ok {
		return rules
	}

	if exeDir != "" {
		if patterns, ok := readIgnoreFile(filepath.Join(exeDir, ignoreFileName)); ok {
			return ignoreRules{anchor: exeDir, patterns: patterns}
		}
	}

	return ignoreRules{anchor: root, patterns: defaultSkipPatterns}
}

// findIgnoreUpward looks for a `.dir-age-ignore` in dir, then dir's parent,
// and so on up to the filesystem root, returning the first one found.
func findIgnoreUpward(dir string) (ignoreRules, bool) {
	current, err := filepath.Abs(dir)
	if err != nil {
		current = dir
	}
	for {
		if patterns, ok := readIgnoreFile(filepath.Join(current, ignoreFileName)); ok {
			return ignoreRules{anchor: current, patterns: patterns}, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ignoreRules{}, false
		}
		current = parent
	}
}

// readIgnoreFile parses a `.dir-age-ignore` file using gitignore syntax: one
// pattern per line, blank lines and "#" comments ignored.
func readIgnoreFile(path string) ([]pattern, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	var patterns []pattern
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if p, ok := parseLine(scanner.Text()); ok {
			patterns = append(patterns, p)
		}
	}
	return patterns, true
}
