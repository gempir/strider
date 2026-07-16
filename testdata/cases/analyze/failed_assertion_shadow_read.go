package analyze_cases

import "fmt"

func inspect(value any) {
	if value, ok := value.(int); ok {
		fmt.Println(value)
	} else {
		fmt.Printf("unexpected type %T", value)
	}
}
