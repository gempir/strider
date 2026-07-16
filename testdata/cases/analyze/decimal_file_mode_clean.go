package analyze_cases

import "os"

func octalMode(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
