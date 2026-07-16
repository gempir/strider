package analyze_cases

import "fmt"

func inspectClean(value any) {
	if typed, ok := value.(int); ok {
		fmt.Println(typed)
	} else {
		fmt.Printf("unexpected type %T", value)
	}
	if value, ok := value.(int); ok {
		fmt.Println(value)
	} else {
		value = 0
		fmt.Println(value)
	}
}
