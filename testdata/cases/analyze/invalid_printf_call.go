package analyze_cases

import "fmt"

func invalidPrintfArgument() string {
	return fmt.Sprintf("count: %d", "many")
}
