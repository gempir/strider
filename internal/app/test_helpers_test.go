package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

func runCLI(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return Run(context.Background(), args, stdin, stdout, stderr)
}

func TestRunRejectsPreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	if code := Run(ctx, []string{
		"check",
	}, strings.NewReader(""), &stdout, &stderr); code != exitError || !strings.Contains(stderr.String(), context.Canceled.Error()) {
		t.Fatalf("exit = %d, stderr = %q; want canceled error", code, stderr.String())
	}
}

func listedSeverity(output, code string) (string, bool) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == code {
			return fields[1], true
		}
	}
	return "", false
}

func stripTerminalStyles(output string) string {
	for _, sequence := range []string{
		"\x1b[0m",
		"\x1b[1;31m",
		"\x1b[1;33m",
		"\x1b[1;34m",
	} {
		output = strings.ReplaceAll(output, sequence, "")
	}
	return output
}

func restoreWorkingDirectory(t *testing.T, directory string) {
	t.Helper()
	if err := os.Chdir(directory); err != nil {
		t.Errorf("restore working directory: %v", err)
	}
}
