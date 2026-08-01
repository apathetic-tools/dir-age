package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDisplayLabel(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	wantForDot := filepath.Base(wd)

	cases := []struct {
		name string
		path string
		want string
	}{
		{"dot resolves to folder name", ".", wantForDot},
		{"dot slash resolves to folder name", "./", wantForDot},
		{"parent dir left as-is", "..", ".."},
		{"relative subdir left as-is", filepath.Join("sub", "dir"), filepath.Join("sub", "dir")},
		{"absolute path left as-is", filepath.Join(wd, "sub"), filepath.Join(wd, "sub")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := displayLabel(c.path); got != c.want {
				t.Errorf("displayLabel(%q) = %q, want %q", c.path, got, c.want)
			}
		})
	}
}

func TestAnalyzeMissingPath(t *testing.T) {
	r := analyze(filepath.Join(t.TempDir(), "does-not-exist"))
	if r.err == nil {
		t.Fatal("expected error for missing path, got nil")
	}
}

func TestAnalyzeDirectory_SkipsDefaultIgnoredDirs(t *testing.T) {
	dir := t.TempDir()
	kept := mustWriteFile(t, dir, filepath.Join("keep", "a.txt"))
	skipped := mustWriteFile(t, dir, filepath.Join("node_modules", "pkg", "index.js"))

	withStubTimes(t, map[string]stubTime{
		kept:    {mod: date(2022, 1, 1), birth: date(2022, 1, 1)},
		skipped: {mod: date(1999, 1, 1), birth: date(1999, 1, 1)}, // would win the min/max if not skipped
	})

	r := analyze(dir)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.fileCount != 1 {
		t.Errorf("fileCount = %d, want 1 (node_modules should be skipped)", r.fileCount)
	}
	if want := date(2022, 1, 1); !r.earliest.Equal(want) {
		t.Errorf("earliest = %v, want %v (node_modules file leaked into aggregation)", r.earliest, want)
	}
}

func TestAnalyzeDirectory_RootItselfNeverSkippedEvenIfNameMatchesList(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "build") // "build" is in the default skip list
	file := mustWriteFile(t, root, "output.txt")

	withStubTimes(t, map[string]stubTime{
		file: {mod: date(2022, 1, 1), birth: date(2022, 1, 1)},
	})

	r := analyze(root)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.fileCount != 1 {
		t.Errorf("fileCount = %d, want 1; the scan root must not be skipped just because its name matches the skip list", r.fileCount)
	}
}

func TestAnalyzeDirectory_CustomIgnoreFileOverridesDefaultsAndIsExcludedItself(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ignoreFileName), []byte("skipme\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// "vendor" is in the built-in default list, but the custom file above
	// only lists "skipme" - so vendor should now be walked, and skipme should
	// be excluded even though it isn't a built-in default.
	vendorFile := mustWriteFile(t, dir, filepath.Join("vendor", "lib.go"))
	skippedFile := mustWriteFile(t, dir, filepath.Join("skipme", "x.txt"))

	withStubTimes(t, map[string]stubTime{
		vendorFile:  {mod: date(2022, 1, 1), birth: date(2022, 1, 1)},
		skippedFile: {mod: date(1999, 1, 1), birth: date(1999, 1, 1)},
	})

	r := analyze(dir)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.fileCount != 1 {
		t.Errorf("fileCount = %d, want 1 (only vendor/lib.go; skipme excluded by custom list, .dir-age-ignore itself never counted)", r.fileCount)
	}
	if want := date(2022, 1, 1); !r.earliest.Equal(want) {
		t.Errorf("earliest = %v, want %v", r.earliest, want)
	}
}

func TestAnalyzeDirectory_AncestorIgnoreFileAppliesToWholeScan(t *testing.T) {
	// The ignore file lives above the scanned root, not inside it.
	ancestor := t.TempDir()
	scanRoot := filepath.Join(ancestor, "project", "target")
	if err := os.WriteFile(filepath.Join(ancestor, ignoreFileName), []byte("skipme\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	skipped := mustWriteFile(t, scanRoot, filepath.Join("skipme", "a.txt"))
	kept := mustWriteFile(t, scanRoot, filepath.Join("keep", "b.txt"))

	withStubTimes(t, map[string]stubTime{
		skipped: {mod: date(1999, 1, 1), birth: date(1999, 1, 1)},
		kept:    {mod: date(2022, 1, 1), birth: date(2022, 1, 1)},
	})

	r := analyze(scanRoot)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.fileCount != 1 {
		t.Errorf("fileCount = %d, want 1 (skipme/ should be excluded via the ancestor's ignore file)", r.fileCount)
	}
	if want := date(2022, 1, 1); !r.earliest.Equal(want) {
		t.Errorf("earliest = %v, want %v", r.earliest, want)
	}
}

func TestAnalyzeDirectory_NestedIgnoreFileScopesToItsOwnSubtreeOnly(t *testing.T) {
	root := t.TempDir()

	// Outside "scoped/": built-in defaults are in effect (no ignore file at
	// root), so node_modules is skipped and an ordinary directory is kept.
	rootNodeModules := mustWriteFile(t, root, filepath.Join("node_modules", "x.txt"))
	otherOnlyhere := mustWriteFile(t, root, filepath.Join("other", "onlyhere", "c.txt"))

	// Inside "scoped/": its own ignore file lists only "onlyhere", fully
	// replacing (not adding to) the inherited defaults for this subtree.
	if err := os.MkdirAll(filepath.Join(root, "scoped"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scoped", ignoreFileName), []byte("onlyhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scopedOnlyhere := mustWriteFile(t, root, filepath.Join("scoped", "onlyhere", "a.txt"))
	scopedNodeModules := mustWriteFile(t, root, filepath.Join("scoped", "node_modules", "b.txt"))

	withStubTimes(t, map[string]stubTime{
		rootNodeModules:   {mod: date(1999, 1, 1), birth: date(1999, 1, 1)},
		otherOnlyhere:     {mod: date(2021, 1, 1), birth: date(2021, 1, 1)},
		scopedOnlyhere:    {mod: date(1998, 1, 1), birth: date(1998, 1, 1)},
		scopedNodeModules: {mod: date(2022, 1, 1), birth: date(2022, 1, 1)},
	})

	r := analyze(root)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}

	// Expect exactly otherOnlyhere and scopedNodeModules to be counted:
	//  - root/node_modules is skipped by the inherited default list.
	//  - other/onlyhere is NOT skipped: "onlyhere" is scoped/'s own rule and
	//    must not leak out to a sibling directory.
	//  - scoped/onlyhere IS skipped: scoped/'s own ignore file applies here.
	//  - scoped/node_modules is NOT skipped: scoped/'s ignore file fully
	//    replaces the inherited defaults, so node_modules is no longer
	//    special inside that subtree.
	if r.fileCount != 2 {
		t.Errorf("fileCount = %d, want 2 (other/onlyhere/c.txt and scoped/node_modules/b.txt)", r.fileCount)
	}
	if want := date(2021, 1, 1); !r.earliest.Equal(want) {
		t.Errorf("earliest = %v, want %v", r.earliest, want)
	}
	if want := date(2022, 1, 1); !r.latest.Equal(want) {
		t.Errorf("latest = %v, want %v", r.latest, want)
	}
}

func TestAnalyzeDirectory_IgnoreFileSkipsFilesByWildcard(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ignoreFileName), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	kept := mustWriteFile(t, dir, "a.txt")
	skipped := mustWriteFile(t, dir, filepath.Join("sub", "debug.log"))

	withStubTimes(t, map[string]stubTime{
		kept:    {mod: date(2022, 1, 1), birth: date(2022, 1, 1)},
		skipped: {mod: date(1999, 1, 1), birth: date(1999, 1, 1)},
	})

	r := analyze(dir)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.fileCount != 1 {
		t.Errorf("fileCount = %d, want 1 (*.log files should be skipped at any depth)", r.fileCount)
	}
	if want := date(2022, 1, 1); !r.earliest.Equal(want) {
		t.Errorf("earliest = %v, want %v (debug.log leaked into aggregation)", r.earliest, want)
	}
}

func TestAnalyzeDirectory_IgnoreFileAnchoredPatternOnlyMatchesAtRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ignoreFileName), []byte("/only-root\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	skippedAtRoot := mustWriteFile(t, dir, filepath.Join("only-root", "a.txt"))
	keptNested := mustWriteFile(t, dir, filepath.Join("sub", "only-root", "b.txt"))

	withStubTimes(t, map[string]stubTime{
		skippedAtRoot: {mod: date(1999, 1, 1), birth: date(1999, 1, 1)},
		keptNested:    {mod: date(2022, 1, 1), birth: date(2022, 1, 1)},
	})

	r := analyze(dir)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.fileCount != 1 {
		t.Errorf("fileCount = %d, want 1 (/only-root should only skip the top-level directory, not sub/only-root)", r.fileCount)
	}
	if want := date(2022, 1, 1); !r.earliest.Equal(want) {
		t.Errorf("earliest = %v, want %v", r.earliest, want)
	}
}

func TestAnalyzeDirectory_IgnoreFileAnchoredPathMatchesExactLocation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ignoreFileName), []byte("sub/skip\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	skipped := mustWriteFile(t, dir, filepath.Join("sub", "skip", "a.txt"))
	kept := mustWriteFile(t, dir, filepath.Join("other", "sub", "skip", "b.txt"))

	withStubTimes(t, map[string]stubTime{
		skipped: {mod: date(1999, 1, 1), birth: date(1999, 1, 1)},
		kept:    {mod: date(2022, 1, 1), birth: date(2022, 1, 1)},
	})

	r := analyze(dir)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.fileCount != 1 {
		t.Errorf("fileCount = %d, want 1 (sub/skip should only match at that exact path, not other/sub/skip)", r.fileCount)
	}
}

func TestAnalyzeDirectory_IgnoreFileNegationReincludesFile(t *testing.T) {
	dir := t.TempDir()
	content := "*.log\n!keep.log\n"
	if err := os.WriteFile(filepath.Join(dir, ignoreFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	kept := mustWriteFile(t, dir, "keep.log")
	skipped := mustWriteFile(t, dir, "debug.log")

	withStubTimes(t, map[string]stubTime{
		kept:    {mod: date(2022, 1, 1), birth: date(2022, 1, 1)},
		skipped: {mod: date(1999, 1, 1), birth: date(1999, 1, 1)},
	})

	r := analyze(dir)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.fileCount != 1 {
		t.Errorf("fileCount = %d, want 1 (keep.log should be re-included by the '!' rule)", r.fileCount)
	}
	if want := date(2022, 1, 1); !r.earliest.Equal(want) {
		t.Errorf("earliest = %v, want %v", r.earliest, want)
	}
}

func TestAnalyzeDirectory_IgnoreFileDirOnlySuffixDoesNotSkipSameNamedFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ignoreFileName), []byte("build/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A file and a directory can't share a name in the same parent, so put
	// them under separate parents but governed by the same ignore file.
	skippedDir := mustWriteFile(t, dir, filepath.Join("dir-variant", "build", "a.txt"))
	keptFile := mustWriteFile(t, dir, filepath.Join("file-variant", "build")) // a plain file named "build"
	_ = skippedDir

	withStubTimes(t, map[string]stubTime{
		keptFile: {mod: date(2022, 1, 1), birth: date(2022, 1, 1)},
	})

	r := analyze(dir)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.fileCount != 1 {
		t.Errorf("fileCount = %d, want 1 (build/ should skip the directory but not a file also named build)", r.fileCount)
	}
}

func TestAnalyzeDirectory_IgnoreFileSkipsSiblingFileAfterSubdirectory(t *testing.T) {
	// Regression guard: a file that comes after a subdirectory at the same
	// level (alphabetically) must still be checked against the correct
	// (immediate parent's) ignore rules, not whatever rules were left on top
	// of the stack from walking that earlier subdirectory.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ignoreFileName), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	inSubdir := mustWriteFile(t, dir, filepath.Join("aaa-sub", "kept.txt"))
	siblingSkipped := mustWriteFile(t, dir, "zzz-debug.log")

	withStubTimes(t, map[string]stubTime{
		inSubdir:       {mod: date(2022, 1, 1), birth: date(2022, 1, 1)},
		siblingSkipped: {mod: date(1999, 1, 1), birth: date(1999, 1, 1)},
	})

	r := analyze(dir)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.fileCount != 1 {
		t.Errorf("fileCount = %d, want 1 (zzz-debug.log should be skipped even though it's visited after aaa-sub/)", r.fileCount)
	}
}
