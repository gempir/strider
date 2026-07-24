//strider:ignore-file cognitive-complexity,function-length
package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGlobalCacheDisableAndClearControls(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "main.go")
	if err := os.WriteFile(sourcePath, []byte("package p\nfunc F( ){ }\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	disabledCache := filepath.Join(root, "disabled-cache")
	t.Setenv("STRIDER_CACHE_DIR", disabledCache)
	var stdout, stderr bytes.Buffer
	exitCode := runFrom(context.Background(), root, []string{
		"--no-cache",
		"--no-config",
		"fmt",
		"--check",
		sourcePath,
	}, bytes.NewReader(nil), &stdout, &stderr)
	if exitCode != exitFindings {
		t.Fatalf("disabled cache exit = %d, stderr = %q", exitCode, stderr.String())
	}
	if _, err := os.Stat(disabledCache); !os.IsNotExist(err) {
		t.Fatalf("disabled cache was created: %v", err)
	}

	cacheDirectory := filepath.Join(root, "cache")
	stdout.Reset()
	stderr.Reset()
	exitCode = runFrom(
		context.Background(),
		root,
		[]string{
			"--cache-dir",
			cacheDirectory,
			"--no-config",
			"fmt",
			"--check",
			sourcePath,
		},
		bytes.NewReader(nil),
		&stdout,
		&stderr,
	)
	if exitCode != exitFindings {
		t.Fatalf("cached exit = %d, stderr = %q", exitCode, stderr.String())
	}
	sentinel := filepath.Join(cacheDirectory, "entries", "v999", "sentinel")
	if err := os.MkdirAll(filepath.Dir(sentinel), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, []byte("old schema"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = runFrom(
		context.Background(),
		root,
		[]string{
			"--cache-dir",
			cacheDirectory,
			"--clear-cache",
			"--no-config",
			"fmt",
			"--check",
			sourcePath,
		},
		bytes.NewReader(nil),
		&stdout,
		&stderr,
	)
	if exitCode != exitFindings {
		t.Fatalf("clear cache exit = %d, stderr = %q", exitCode, stderr.String())
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("clear retained old schema entry: %v", err)
	}
}
