package analyze_cases

import (
	"net"
	"strconv"
)

func joinedURLHostPort(host string, port int) string {
	return "https://" + net.JoinHostPort(host, strconv.Itoa(port)) + "/path"
}
