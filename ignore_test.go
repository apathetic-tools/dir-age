package main

import (
	"os"
	"path/filepath"
	"testing"
)

// --- pattern parsing ---

func TestParseLine_BlankAndComments(t *testing.T) {
	cases := []string{"", "   ", "# a comment", "  # indented comment"}
	for _, line := range cases {
		if _, ok := parseLine(line); ok {
			t.Errorf("parseLine(%q) = ok, want blank/comment to be skipped", line)
		}
	}
}

func TestParseLine_TrailingSpaceTrimmed(t *testing.T) {
	p, ok := parseLine("build   ")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got := p.segments[len(p.segments)-1]; got != "build" {
		t.Errorf("got segment %q, want %q (trailing whitespace should be trimmed)", got, "build")
	}
}

func TestParseLine_Negation(t *testing.T) {
	p, ok := parseLine("!keep.txt")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !p.negate {
		t.Error("expected negate=true for a line starting with !")
	}
}

func TestParseLine_EscapedLeadingBangIsLiteral(t *testing.T) {
	p, ok := parseLine(`\!literal`)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if p.negate {
		t.Error("expected negate=false for an escaped leading '!'")
	}
	if got := p.segments[len(p.segments)-1]; got != "!literal" {
		t.Errorf("got segment %q, want %q", got, "!literal")
	}
}

func TestParseLine_TrailingSlashIsDirOnly(t *testing.T) {
	p, ok := parseLine("build/")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !p.dirOnly {
		t.Error("expected dirOnly=true for a trailing slash")
	}
}

func TestParseLine_NoSlashIsUnanchored(t *testing.T) {
	p, ok := parseLine("*.log")
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := []string{"**", "*.log"}
	if len(p.segments) != 2 || p.segments[0] != want[0] || p.segments[1] != want[1] {
		t.Errorf("got segments %v, want %v (a slash-free pattern should match at any depth)", p.segments, want)
	}
}

func TestParseLine_LeadingSlashAnchorsToRoot(t *testing.T) {
	p, ok := parseLine("/only-root")
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := []string{"only-root"}
	if len(p.segments) != 1 || p.segments[0] != want[0] {
		t.Errorf("got segments %v, want %v (leading slash should anchor without an implicit '**')", p.segments, want)
	}
}

func TestParseLine_MidSlashAnchors(t *testing.T) {
	p, ok := parseLine("sub/dir")
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := []string{"sub", "dir"}
	if len(p.segments) != 2 || p.segments[0] != want[0] || p.segments[1] != want[1] {
		t.Errorf("got segments %v, want %v (a mid-pattern slash should anchor without an implicit '**')", p.segments, want)
	}
}

// --- pattern matching ---

func TestMatchAny_PlainNameMatchesAtAnyDepth(t *testing.T) {
	patterns := parseLines([]string{"node_modules"})
	cases := []string{"node_modules", "sub/node_modules", "a/b/c/node_modules"}
	for _, rel := range cases {
		if !matchAny(patterns, rel, true) {
			t.Errorf("matchAny(%q) = false, want true", rel)
		}
	}
	if matchAny(patterns, "node_modules_extra", true) {
		t.Error("matchAny(node_modules_extra) = true, want false (must match whole segment, not a prefix)")
	}
}

func TestMatchAny_Wildcard(t *testing.T) {
	patterns := parseLines([]string{"*.log"})
	if !matchAny(patterns, "debug.log", false) {
		t.Error("expected *.log to match debug.log")
	}
	if !matchAny(patterns, "nested/dir/debug.log", false) {
		t.Error("expected *.log to match a nested debug.log (unanchored pattern)")
	}
	if matchAny(patterns, "debug.txt", false) {
		t.Error("expected *.log not to match debug.txt")
	}
	if matchAny(patterns, "logs/debug.log/extra", false) {
		t.Error("expected *.log not to match across a '/' within a single segment")
	}
}

func TestMatchAny_QuestionMarkAndCharClass(t *testing.T) {
	patterns := parseLines([]string{"file?.txt", "[abc].txt"})
	if !matchAny(patterns, "file1.txt", false) {
		t.Error("expected file?.txt to match file1.txt")
	}
	if matchAny(patterns, "file12.txt", false) {
		t.Error("expected file?.txt not to match file12.txt")
	}
	if !matchAny(patterns, "a.txt", false) {
		t.Error("expected [abc].txt to match a.txt")
	}
	if matchAny(patterns, "d.txt", false) {
		t.Error("expected [abc].txt not to match d.txt")
	}
}

func TestMatchAny_LeadingSlashAnchorsToTopLevelOnly(t *testing.T) {
	patterns := parseLines([]string{"/only-root"})
	if !matchAny(patterns, "only-root", true) {
		t.Error("expected /only-root to match at the top level")
	}
	if matchAny(patterns, "sub/only-root", true) {
		t.Error("expected /only-root not to match when nested (leading slash anchors to root)")
	}
}

func TestMatchAny_MidPathAnchorsExactPath(t *testing.T) {
	patterns := parseLines([]string{"sub/dir"})
	if !matchAny(patterns, "sub/dir", true) {
		t.Error("expected sub/dir to match sub/dir")
	}
	if matchAny(patterns, "dir", true) {
		t.Error("expected sub/dir not to match a bare dir at the top level")
	}
	if matchAny(patterns, "other/sub/dir", true) {
		t.Error("expected sub/dir not to match when nested deeper than the anchor")
	}
}

func TestMatchAny_DoubleStarMatchesAnyDepth(t *testing.T) {
	patterns := parseLines([]string{"**/target/**"})
	if !matchAny(patterns, "a/b/target/c/d", false) {
		t.Error("expected **/target/** to match a/b/target/c/d")
	}
	if !matchAny(patterns, "target/c", false) {
		t.Error("expected **/target/** to match target/c (leading ** can match zero segments)")
	}
}

func TestMatchAny_DirOnlyDoesNotMatchFiles(t *testing.T) {
	patterns := parseLines([]string{"build/"})
	if matchAny(patterns, "build", false) {
		t.Error("expected build/ not to match a file named build")
	}
	if !matchAny(patterns, "build", true) {
		t.Error("expected build/ to match a directory named build")
	}
}

func TestMatchAny_UnanchoredWithoutTrailingSlashMatchesFilesAndDirs(t *testing.T) {
	patterns := parseLines([]string{"build"})
	if !matchAny(patterns, "build", false) {
		t.Error("expected build to match a file named build")
	}
	if !matchAny(patterns, "build", true) {
		t.Error("expected build to match a directory named build")
	}
}

func TestMatchAny_LaterNegationReincludes(t *testing.T) {
	patterns := parseLines([]string{"*.log", "!keep.log"})
	if matchAny(patterns, "keep.log", false) {
		t.Error("expected keep.log to be re-included by the later '!' rule")
	}
	if !matchAny(patterns, "debug.log", false) {
		t.Error("expected debug.log to still be excluded")
	}
}

func TestMatchAny_LaterPositiveReExcludesAfterNegation(t *testing.T) {
	// Order matters: the last matching pattern wins either direction.
	patterns := parseLines([]string{"!keep.log", "*.log"})
	if !matchAny(patterns, "keep.log", false) {
		t.Error("expected keep.log to be excluded again since '*.log' comes after the negation")
	}
}

// --- file/precedence resolution ---

func TestReadIgnoreFile_ParsesCommentsBlankLinesAndSpacing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ignoreFileName)
	content := "# leading comment\n\nnode_modules\ncustom   \n  spaced  \n*.log\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := readIgnoreFile(path)
	if !ok {
		t.Fatal("expected ok=true for an existing file")
	}
	if len(got) != 4 {
		t.Fatalf("got %d patterns, want 4: %+v", len(got), got)
	}
	if !matchAny(got, "node_modules", true) {
		t.Error("expected node_modules pattern to be parsed")
	}
	if !matchAny(got, "custom", true) {
		t.Error("expected custom pattern to be parsed (trailing spaces trimmed)")
	}
	if !matchAny(got, "spaced", true) {
		t.Error("expected spaced pattern to be parsed (leading/trailing spaces trimmed)")
	}
	if !matchAny(got, "debug.log", false) {
		t.Error("expected *.log pattern to be parsed")
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
	if got.anchor != root {
		t.Errorf("anchor = %q, want %q", got.anchor, root)
	}
	if !matchAny(got.patterns, "root-custom", true) || matchAny(got.patterns, "exe-custom", true) {
		t.Errorf("got patterns %+v, want only root-custom (root ignore file should take precedence)", got.patterns)
	}
}

func TestResolveSkipDirsFrom_FallsBackToExecutableDirIgnoreFile(t *testing.T) {
	root := t.TempDir() // no ignore file here
	exeDir := t.TempDir()
	writeIgnoreFile(t, exeDir, "exe-custom")

	got := resolveSkipDirsFrom(root, exeDir)
	if got.anchor != exeDir {
		t.Errorf("anchor = %q, want %q", got.anchor, exeDir)
	}
	if !matchAny(got.patterns, "exe-custom", true) {
		t.Errorf("got patterns %+v, want exe-custom", got.patterns)
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
	if got.anchor != exeDir {
		t.Errorf("anchor = %q, want %q", got.anchor, exeDir)
	}
	if !matchAny(got.patterns, "exe-custom", true) {
		t.Errorf("got patterns %+v, want exe-custom (should fall back to the executable-adjacent file)", got.patterns)
	}
}

func TestResolveSkipDirsFrom_FallsBackToBuiltInDefaults(t *testing.T) {
	got := resolveSkipDirsFrom(t.TempDir(), t.TempDir())
	if len(got.patterns) != len(defaultSkipPatterns) {
		t.Errorf("got %d patterns, want built-in default count %d", len(got.patterns), len(defaultSkipPatterns))
	}
	if !matchAny(got.patterns, "node_modules", true) {
		t.Error("expected built-in defaults to include node_modules")
	}
}

func TestResolveSkipDirsFrom_EmptyExecutableDirSkipsThatFallback(t *testing.T) {
	got := resolveSkipDirsFrom(t.TempDir(), "")
	if !matchAny(got.patterns, "node_modules", true) {
		t.Error("expected built-in defaults to include node_modules")
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
	if got.anchor != parent {
		t.Errorf("anchor = %q, want %q", got.anchor, parent)
	}
	if !matchAny(got.patterns, "ancestor-rule", true) {
		t.Errorf("got patterns %+v, want ancestor-rule", got.patterns)
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
	if !matchAny(got.patterns, "grandparent-rule", true) {
		t.Errorf("got patterns %+v, want grandparent-rule", got.patterns)
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
	if !matchAny(got.patterns, "near-rule", true) || matchAny(got.patterns, "far-rule", true) {
		t.Errorf("got patterns %+v, want only near-rule (the nearer file should win over the more distant one)", got.patterns)
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
	if got.anchor != parent {
		t.Errorf("anchor = %q, want %q", got.anchor, parent)
	}
	if !matchAny(got.patterns, "ancestor-rule", true) {
		t.Errorf("got patterns %+v, want ancestor-rule", got.patterns)
	}
}

func writeIgnoreFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ignoreFileName), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
