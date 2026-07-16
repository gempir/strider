package analyze_cases

import "fmt"

func formattedURLHostPort(host string, port int) string {
	return fmt.Sprintf("https://%s:%d/path", host, port)
}
