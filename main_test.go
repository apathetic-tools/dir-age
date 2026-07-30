package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestAnalyzeSingleFile(t *testing.T) {
	dir := t.TempDir()
	file := mustWriteFile(t, dir, "a.txt")

	mod := date(2020, 1, 2)
	birth := date(2019, 1, 1)
	withStubTimes(t, map[string]stubTime{file: {mod: mod, birth: birth}})

	r := analyze(file)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.fileCount != 1 {
		t.Errorf("fileCount = %d, want 1", r.fileCount)
	}
	if !r.latest.Equal(mod) {
		t.Errorf("latest = %v, want %v", r.latest, mod)
	}
	if !r.earliest.Equal(birth) {
		t.Errorf("earliest = %v, want %v", r.earliest, birth)
	}
}

func TestAnalyzeDirectory_AggregatesAcrossNestedFiles(t *testing.T) {
	dir := t.TempDir()
	a := mustWriteFile(t, dir, "a.txt")
	b := mustWriteFile(t, dir, filepath.Join("sub", "b.txt"))
	c := mustWriteFile(t, dir, filepath.Join("sub", "deeper", "c.txt"))

	withStubTimes(t, map[string]stubTime{
		a: {mod: date(2021, 1, 1), birth: date(2020, 1, 1)},
		b: {mod: date(2022, 6, 1), birth: date(2020, 6, 1)},
		c: {mod: date(2020, 3, 1), birth: date(2019, 1, 1)}, // earliest birth, but not earliest mod
	})

	r := analyze(dir)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.fileCount != 3 {
		t.Errorf("fileCount = %d, want 3", r.fileCount)
	}
	if want := date(2019, 1, 1); !r.earliest.Equal(want) {
		t.Errorf("earliest = %v, want %v", r.earliest, want)
	}
	if want := date(2022, 6, 1); !r.latest.Equal(want) {
		t.Errorf("latest = %v, want %v", r.latest, want)
	}
}

func TestAnalyzeDirectory_FoldersDoNotAffectTimes(t *testing.T) {
	dir := t.TempDir()
	file := mustWriteFile(t, dir, filepath.Join("sub", "a.txt"))

	fileTime := date(2021, 5, 5)
	withStubTimes(t, map[string]stubTime{file: {mod: fileTime, birth: fileTime}})

	// Give the directories themselves absurd real mtimes; if a folder's own
	// timestamp ever leaked into the aggregation, these would win.
	future := time.Now().Add(24 * time.Hour)
	if err := os.Chtimes(dir, future, future); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(dir, "sub"), future, future); err != nil {
		t.Fatal(err)
	}

	r := analyze(dir)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if !r.earliest.Equal(fileTime) || !r.latest.Equal(fileTime) {
		t.Errorf("earliest/latest = %v/%v, want both %v (folder times must be ignored)", r.earliest, r.latest, fileTime)
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

// --- test helpers ---

type stubTime struct {
	mod, birth time.Time
}

// withStubTimes replaces timesForPath for the duration of the test so file
// times can be controlled deterministically, independent of what the
// underlying OS/filesystem actually supports for birth time.
func withStubTimes(t *testing.T, stub map[string]stubTime) {
	t.Helper()
	orig := timesForPath
	timesForPath = func(path string) (time.Time, time.Time, error) {
		if st, ok := stub[path]; ok {
			return st.mod, st.birth, nil
		}
		return orig(path)
	}
	t.Cleanup(func() { timesForPath = orig })
}

func mustWriteFile(t *testing.T, root, rel string) string {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return full
}

func date(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}
