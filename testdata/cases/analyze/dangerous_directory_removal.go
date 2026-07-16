package analyze_cases

import "os"

func removeSharedTemporaryDirectory() error {
	directory := os.TempDir()
	return os.RemoveAll(directory)
}
