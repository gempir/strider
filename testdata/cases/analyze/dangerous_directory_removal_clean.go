package analyze_cases

import (
	"os"
	"path/filepath"
)

func removeTemporaryChild() error {
	directory := filepath.Join(os.TempDir(), "application")
	return os.RemoveAll(directory)
}
