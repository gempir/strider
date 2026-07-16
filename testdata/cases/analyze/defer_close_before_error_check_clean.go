package analyze_cases

import "os"

func checkedBeforeDeferredClose(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return nil
}
