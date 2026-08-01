package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type result struct {
	path      string
	label     string
	err       error
	earliest  time.Time
	latest    time.Time
	fileCount int
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <path> [path...]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Estimates when a directory's contents were created and last updated,")
		fmt.Fprintln(os.Stderr, "based on the birth/modified times of files inside it.")
		fmt.Fprintln(os.Stderr, "Drag a folder onto the executable, or pass one or more paths on the command line.")
	}
	flag.Parse()

	paths := flag.Args()
	if len(paths) == 0 {
		flag.Usage()
		pauseIfDoubleClicked()
		os.Exit(1)
	}

	for i, p := range paths {
		if i > 0 {
			fmt.Println()
		}
		printResult(analyze(p))
	}

	pauseIfDoubleClicked()
}

func analyze(root string) result {
	r := result{path: root, label: displayLabel(root)}

	info, err := os.Stat(root)
	if err != nil {
		r.err = err
		return r
	}

	if !info.IsDir() {
		r.fileCount = 1
		mod, birth, err := timesForPath(root)
		if err != nil {
			r.err = err
			return r
		}
		r.latest = mod
		r.earliest = birth
		return r
	}

	// The base ignore rules for the whole scan (see resolveSkipDirs), tracked
	// alongside the directory it applies from so a `.dir-age-ignore` found
	// deeper in the tree can override it for just that subtree, the same way
	// .gitignore cascades: nested rules win locally without affecting
	// siblings or the parent scan.
	type ignoreFrame struct {
		dir   string
		rules ignoreRules
	}
	stack := []ignoreFrame{{dir: root, rules: resolveSkipDirs(root)}}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}

		for len(stack) > 1 && stack[len(stack)-1].dir != filepath.Dir(path) {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]

		isDir := d.IsDir()
		relPath, _ := filepath.Rel(parent.rules.anchor, path)
		if matchAny(parent.rules.patterns, relPath, isDir) {
			if isDir {
				return fs.SkipDir
			}
			return nil
		}

		if isDir {
			effective := parent.rules
			if patterns, ok := readIgnoreFile(filepath.Join(path, ignoreFileName)); ok {
				effective = ignoreRules{anchor: path, patterns: patterns}
			}
			stack = append(stack, ignoreFrame{dir: path, rules: effective})
			return nil
		}
		if d.Name() == ignoreFileName {
			return nil
		}
		mod, birth, err := timesForPath(path)
		if err != nil {
			return nil
		}

		r.fileCount++
		if r.earliest.IsZero() || birth.Before(r.earliest) {
			r.earliest = birth
		}
		if r.latest.IsZero() || mod.After(r.latest) {
			r.latest = mod
		}
		return nil
	})
	if err != nil {
		r.err = err
	}
	return r
}

// displayLabel returns what to print for a path. For "." (the current
// directory, however written, e.g. "./" or ".\"), it resolves to the
// directory's own name rather than printing the uninformative ".".
func displayLabel(path string) string {
	if filepath.Clean(path) != "." {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return filepath.Base(abs)
}

func printResult(r result) {
	fmt.Println(r.label)
	if r.err != nil {
		fmt.Printf("  error: %v\n", r.err)
		return
	}
	if r.fileCount == 0 {
		fmt.Println("  (no files found)")
		return
	}
	fmt.Printf("  likely created: %s\n", r.earliest.Format("2006-01-02 15:04:05"))
	fmt.Printf("  last updated:   %s\n", r.latest.Format("2006-01-02 15:04:05"))
}
