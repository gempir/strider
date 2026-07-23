package formatter

import (
	"bytes"
	"slices"
	"testing"

	"github.com/gempir/strider/internal/cst"
)

func parseSafetyTree(t *testing.T, source string) *cst.Tree {
	t.Helper()
	tree, err := cst.Parse("safety.go", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestFingerprintTreeIgnoresImportOrderAndLayoutTokens(t *testing.T) {
	original := parseSafetyTree(t, "package p\nimport (\"example.net/x\"; \"fmt\")\nvar values = []int{1, 2}\n")
	formatted := parseSafetyTree(t, "package p\n\nimport (\n\t\"fmt\"\n\n\t\"example.net/x\"\n)\n\nvar values = []int{1, 2}\n")
	originalFingerprint := fingerprintTree(original)
	formattedFingerprint := fingerprintTree(formatted)
	if !slices.Equal(originalFingerprint.imports, formattedFingerprint.imports) {
		t.Fatalf("import fingerprints differ: %q != %q", originalFingerprint.imports, formattedFingerprint.imports)
	}
	if !bytes.Equal(originalFingerprint.syntax, formattedFingerprint.syntax) {
		t.Fatal("layout-only changes altered the syntax fingerprint")
	}
	if err := equivalentTrees(original, formatted); err != nil {
		t.Fatalf("equivalent trees rejected: %v", err)
	}
}

func TestEquivalentTreesRejectsSyntaxAndCommentChanges(t *testing.T) {
	original := parseSafetyTree(t, "package p\n// value is stable\nvar value = 1\n")
	changedSyntax := parseSafetyTree(t, "package p\n// value is stable\nvar value = 2\n")
	if err := equivalentTrees(original, changedSyntax); err == nil {
		t.Fatal("syntax change passed the formatter safety check")
	}
	changedComment := parseSafetyTree(t, "package p\n// value changed\nvar value = 1\n")
	if err := equivalentTrees(original, changedComment); err == nil {
		t.Fatal("comment change passed the formatter safety check")
	}
}

func TestCommentContentsForSafetyIgnoresDocFormattingMarkers(t *testing.T) {
	comments := []cst.Comment{
		{
			Text: "//",
		},
		{
			Text: "// # Heading",
		},
		{
			Text: "//   indented detail",
		},
	}
	got := commentContentsForSafety(comments)
	want := []string{
		"// Heading",
		"// indented detail",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("comment contents = %q, want %q", got, want)
	}
}
