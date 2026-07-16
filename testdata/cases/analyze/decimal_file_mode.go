package analyze_cases

import "os"

func decimalMode(path string, data []byte) error {
	return os.WriteFile(path, data, 644)
}
