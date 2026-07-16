package analyze_cases

import "os"

func deferCloseBeforeErrorCheck(path string) error {
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return err
	}
	return nil
}
