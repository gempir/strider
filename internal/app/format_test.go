package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/filewrite"
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/ui"
)

func TestFormatStdin(t *testing.T) {
	stdin := strings.NewReader("package p\nfunc F( ){return}\n")
	var stdout, stderr bytes.Buffer
	if code := runCLI([]string{
		"fmt",
		"--stdin",
	}, stdin, &stdout, &stderr); code != exitSuccess {
		t.Fatalf("exit %d, stderr %s", code, stderr.String())
	}
	if want := "package p\n\nfunc F() {\n\treturn\n}\n"; stdout.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", stdout.String(), want)
	}
}

func TestFormatWithoutPathsScansCurrentDirectory(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(filename, []byte("package p\nfunc F( ){return}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		restoreWorkingDirectory(t, previous)
	})
	var stdout, stderr bytes.Buffer
	if code := runCLI([]string{
		"fmt",
		"--check",
	}, strings.NewReader("ignored stdin"), &stdout, &stderr); code != exitFindings {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "main.go") {
		t.Fatalf("current-directory file not reported: %q", stdout.String())
	}
}

func TestFormatBatchDoesNotWriteWhenAnyFileDoesNotParse(t *testing.T) {
	root := t.TempDir()
	good := filepath.Join(root, "good.go")
	bad := filepath.Join(root, "bad.go")
	original := []byte("package p\nfunc F( ){return}\n")
	if err := os.WriteFile(good, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("package p\nfunc F( {\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runCLI([]string{
		"fmt",
		"--write",
		root,
	}, strings.NewReader(""), &stdout, &stderr); code != exitError {
		t.Fatalf("exit %d, stderr %s", code, stderr.String())
	}
	after, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, original) {
		t.Fatalf("supported file changed despite batch failure:\n%s", after)
	}
}

func TestFormatBatchReportsFirstFilenameError(t *testing.T) {
	root := t.TempDir()
	for name, source := range map[string]string{
		"a.go": "package p\nfunc A( {\n",
		"z.go": "package p\nfunc Z( {\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	code := runCLI([]string{
		"fmt",
		"--check",
		root,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitError || !strings.Contains(stderr.String(), "a.go") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "z.go") {
		t.Fatalf("reported a later file error: %q", stderr.String())
	}
}

func TestFormatBatchReportsChangesInFilenameOrder(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		"z.go",
		"a.go",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package p\nfunc F( ){return}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	code := runCLI([]string{
		"fmt",
		"--check",
		root,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	a := strings.Index(stdout.String(), "a.go")
	z := strings.Index(stdout.String(), "z.go")
	if a < 0 || z < 0 || a >= z {
		t.Fatalf("changes are not filename ordered: %q", stdout.String())
	}
}

func TestFormatCheckDiffAndWrite(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "main.go")
	original := []byte("package p\nfunc F( ){return}\n")
	if err := os.WriteFile(filename, original, 0o640); err != nil {
		t.Fatal(err)
	}
	originalInfo, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		flag string
		text string
	}{
		{
			flag: "--check",
			text: "main.go",
		},
		{
			flag: "--diff",
			text: "--- ",
		},
	} {
		assertFormatReadOnly(t, filename, original, test.flag, test.text)
	}
	var stdout, stderr bytes.Buffer
	if code := runCLI([]string{
		"fmt",
		"--write",
		filename,
	}, strings.NewReader(""), &stdout, &stderr); code != exitSuccess {
		t.Fatalf("write: exit %d, stderr %q", code, stderr.String())
	}
	after, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != "package p\n\nfunc F() {\n\treturn\n}\n" {
		t.Fatalf("unexpected written source:\n%s", after)
	}
	info, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != originalInfo.Mode().Perm() {
		t.Fatalf("mode changed from %v to %v", originalInfo.Mode().Perm(), info.Mode().Perm())
	}
}

func TestFormatDiffPrintsContextHunks(t *testing.T) {
	before := []byte("line-1\nline-2\nline-3\nline-4\nline-5\nline-6\nline-7\nline-8\nline-9\nline-10\n")
	after := []byte("line-1\nline-2\nline-3\nline-4\nchanged-5\nline-6\nline-7\nline-8\nline-9\nline-10\n")
	var output bytes.Buffer
	printDiff(&output, "sample.go", before, after, ui.NewPalette(&output, ui.ColorNever))
	text := output.String()
	for _, wanted := range []string{
		"@@ -2,7 +2,7 @@",
		" line-4\n",
		"-line-5\n",
		"+changed-5\n",
	} {
		if !strings.Contains(text, wanted) {
			t.Fatalf("diff missing %q:\n%s", wanted, text)
		}
	}
	for _, outsideContext := range []string{
		" line-1\n",
		" line-9\n",
		" line-10\n",
	} {
		if strings.Contains(text, outsideContext) {
			t.Fatalf("diff contains out-of-hunk context %q:\n%s", outsideContext, text)
		}
	}
}

func TestLineDiffReconstructsBothInputs(t *testing.T) {
	for name, sources := range map[string][2]string{
		"add at start": {
			"second\nthird\n",
			"first\nsecond\nthird\n",
		},
		"add at end": {
			"first\nsecond\n",
			"first\nsecond\nthird\n",
		},
		"delete middle": {
			"first\nremove\nthird\n",
			"first\nthird\n",
		},
		"replace all": {
			"old-1\nold-2\n",
			"new-1\nnew-2\n",
		},
		"line endings": {
			"first\r\nsecond\r\n",
			"first\nsecond\n",
		},
		"final newline": {
			"same",
			"same\n",
		},
		"empty to content": {
			"",
			"first\n",
		},
	} {
		t.Run(
			name,
			func(t *testing.T) {
				before := splitSourceLines([]byte(sources[0]))
				after := splitSourceLines([]byte(sources[1]))
				operations := lineDiff(before, after)
				var reconstructedBefore, reconstructedAfter []sourceLine
				for _, operation := range operations {
					if operation.kind != diffAdd {
						reconstructedBefore = append(reconstructedBefore, operation.line)
					}
					if operation.kind != diffRemove {
						reconstructedAfter = append(reconstructedAfter, operation.line)
					}
				}
				if !slices.Equal(reconstructedBefore, before) || !slices.Equal(reconstructedAfter, after) {
					t.Fatalf("operations %#v reconstruct before %#v and after %#v", operations, reconstructedBefore, reconstructedAfter)
				}
			},
		)
	}
}

func FuzzLineDiffReconstructsSource(f *testing.F) {
	f.Add("first\nsecond\n", "first\nchanged\n")
	f.Add("first\r\n", "first\n")
	f.Add("", "content")
	f.Fuzz(
		func(t *testing.T, beforeText, afterText string) {
			if len(beforeText)+len(afterText) > 16_384 {
				return
			}
			before := splitSourceLines([]byte(beforeText))
			after := splitSourceLines([]byte(afterText))
			operations := lineDiff(before, after)
			reconstructed := make([]sourceLine, 0, len(after))
			for _, operation := range operations {
				if operation.kind != diffRemove {
					reconstructed = append(reconstructed, operation.line)
				}
			}
			if got := joinSourceLines(reconstructed); got != afterText {
				t.Fatalf("operations reconstruct %q, want %q", got, afterText)
			}
		},
	)
}

func joinSourceLines(lines []sourceLine) string {
	var result strings.Builder
	for _, line := range lines {
		result.WriteString(line.text)
		if line.carriage {
			result.WriteByte('\r')
		}
		if line.newline {
			result.WriteByte('\n')
		}
	}
	return result.String()
}

func TestFormatWriterRejectsStaleSource(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "sample.go")
	original := []byte("package sample\nfunc F( ){return}\n")
	if err := os.WriteFile(filename, original, 0o640); err != nil {
		t.Fatal(err)
	}
	changed := []byte("package sample\n\nfunc changed() {}\n")
	if err := os.WriteFile(filename, changed, 0o640); err != nil {
		t.Fatal(err)
	}
	err := writeFormattedFiles(
		[]formattedFile{
			{
				filename: filename,
				original: original,
				result: formatter.Result{
					Source:  []byte("package sample\n\nfunc F() {\n\treturn\n}\n"),
					Changed: true,
				},
			},
		},
	)
	if !errors.Is(err, filewrite.ErrStale) {
		t.Fatalf("write error = %v, want ErrStale", err)
	}
	contents, readErr := os.ReadFile(filename)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(contents, changed) {
		t.Fatalf("stale write replaced current contents:\n%s", contents)
	}
}

func assertFormatReadOnly(t *testing.T, filename string, original []byte, flag, expected string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runCLI([]string{
		"fmt",
		flag,
		filename,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings || !strings.Contains(stdout.String(), expected) {
		t.Fatalf("%s: exit %d, stdout %q, stderr %q", flag, code, stdout.String(), stderr.String())
	}
	after, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, original) {
		t.Fatalf("%s changed the source", flag)
	}
}
