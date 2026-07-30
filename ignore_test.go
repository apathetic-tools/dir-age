package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadIgnoreFile_ParsesCommentsBlankLinesAndSpacing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ignoreFileName)
	content := "# leading comment\n\nnode_modules\ncustom # trailing comment\n  spaced  \n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := readIgnoreFile(path)
	if !ok {
		t.Fatal("expected ok=true for an existing file")
	}
	want := map[string]bool{"node_modules": true, "custom": true, "spaced": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestReadIgnoreFile_MissingFile(t *testing.T) {
	_, ok := readIgnoreFile(filepath.Join(t.TempDir(), "nope"))
	if ok {
		t.Error("expected ok=false for a missing file")
	}
}

func TestResolveSkipDirsFrom_PrefersRootIgnoreFileOverExecutableDir(t *testing.T) {
	root := t.TempDir()
	exeDir := t.TempDir()
	writeIgnoreFile(t, root, "root-custom")
	writeIgnoreFile(t, exeDir, "exe-custom")

	got := resolveSkipDirsFrom(root, exeDir)
	want := map[string]bool{"root-custom": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v (root ignore file should take precedence)", got, want)
	}
}

func TestResolveSkipDirsFrom_FallsBackToExecutableDirIgnoreFile(t *testing.T) {
	root := t.TempDir() // no ignore file here
	exeDir := t.TempDir()
	writeIgnoreFile(t, exeDir, "exe-custom")

	got := resolveSkipDirsFrom(root, exeDir)
	want := map[string]bool{"exe-custom": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestResolveSkipDirsFrom_FallsBackToExecutableDirWhenNoAncestorFileExists
// makes explicit that the executable-adjacent fallback only kicks in once
// the upward search genuinely finds nothing — not just at the scanned path
// itself, but at any of its ancestors within the constructed hierarchy.
func TestResolveSkipDirsFrom_FallsBackToExecutableDirWhenNoAncestorFileExists(t *testing.T) {
	root := t.TempDir() // has no .dir-age-ignore anywhere above this point
	scanned := filepath.Join(root, "project", "target")
	if err := os.MkdirAll(scanned, 0o755); err != nil {
		t.Fatal(err)
	}
	exeDir := t.TempDir()
	writeIgnoreFile(t, exeDir, "exe-custom")

	got := resolveSkipDirsFrom(scanned, exeDir)
	want := map[string]bool{"exe-custom": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v (should fall back to the executable-adjacent file since neither the scanned path nor its ancestors have one)", got, want)
	}
}

func TestResolveSkipDirsFrom_FallsBackToBuiltInDefaults(t *testing.T) {
	got := resolveSkipDirsFrom(t.TempDir(), t.TempDir())
	if !reflect.DeepEqual(got, defaultSkipDirs) {
		t.Errorf("got %v, want built-in defaults %v", got, defaultSkipDirs)
	}
}

func TestResolveSkipDirsFrom_EmptyExecutableDirSkipsThatFallback(t *testing.T) {
	got := resolveSkipDirsFrom(t.TempDir(), "")
	if !reflect.DeepEqual(got, defaultSkipDirs) {
		t.Errorf("got %v, want built-in defaults %v", got, defaultSkipDirs)
	}
}

func TestFindIgnoreUpward_FindsFileInImmediateParent(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	writeIgnoreFile(t, parent, "ancestor-rule")

	got, ok := findIgnoreUpward(child)
	if !ok {
		t.Fatal("expected to find the parent's ignore file")
	}
	want := map[string]bool{"ancestor-rule": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFindIgnoreUpward_FindsFileSeveralLevelsUp(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	writeIgnoreFile(t, root, "grandparent-rule")

	got, ok := findIgnoreUpward(deep)
	if !ok {
		t.Fatal("expected to find the ancestor's ignore file several levels up")
	}
	want := map[string]bool{"grandparent-rule": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFindIgnoreUpward_PrefersNearerAncestor(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	writeIgnoreFile(t, root, "far-rule")
	writeIgnoreFile(t, child, "near-rule")

	got, ok := findIgnoreUpward(child)
	if !ok {
		t.Fatal("expected to find an ignore file")
	}
	want := map[string]bool{"near-rule": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v (the nearer file should win over the more distant one)", got, want)
	}
}

func TestResolveSkipDirsFrom_SearchesAncestorsWhenScannedDirHasNoOwnFile(t *testing.T) {
	parent := t.TempDir()
	scanned := filepath.Join(parent, "project", "target")
	if err := os.MkdirAll(scanned, 0o755); err != nil {
		t.Fatal(err)
	}
	writeIgnoreFile(t, parent, "ancestor-rule")

	got := resolveSkipDirsFrom(scanned, "")
	want := map[string]bool{"ancestor-rule": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func writeIgnoreFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ignoreFileName), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
