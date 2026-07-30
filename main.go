package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/djherbis/times"
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

	// The base skip list for the whole scan (see resolveSkipDirs), tracked
	// alongside the directory it applies from so a `.dir-age-ignore` found
	// deeper in the tree can override it for just that subtree, the same way
	// .gitignore cascades: nested rules win locally without affecting
	// siblings or the parent scan.
	type ignoreFrame struct {
		dir  string
		skip map[string]bool
	}
	stack := []ignoreFrame{{dir: root, skip: resolveSkipDirs(root)}}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path == root {
				return nil
			}

			for len(stack) > 1 && stack[len(stack)-1].dir != filepath.Dir(path) {
				stack = stack[:len(stack)-1]
			}
			parent := stack[len(stack)-1]

			if parent.skip[d.Name()] {
				return fs.SkipDir
			}

			effective := parent.skip
			if names, ok := readIgnoreFile(filepath.Join(path, ignoreFileName)); ok {
				effective = names
			}
			stack = append(stack, ignoreFrame{dir: path, skip: effective})
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

// timesForPath is overridden in tests to supply deterministic times without
// depending on real filesystem/platform birth-time behavior.
var timesForPath = fileTimes

// fileTimes returns a file's modified time and its best-available creation
// ("birth") time, falling back to modified time on filesystems that don't
// track birth time (e.g. most Linux filesystems).
func fileTimes(path string) (mod, birth time.Time, err error) {
	t, err := times.Stat(path)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	mod = t.ModTime()
	if t.HasBirthTime() {
		birth = t.BirthTime()
	} else {
		birth = mod
	}
	return mod, birth, nil
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
