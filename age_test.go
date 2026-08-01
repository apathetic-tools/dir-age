package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
