package workspace

import (
	"strconv"
	"strings"
)

func join(sep string, parts ...string) string {
	return strings.Join(parts, sep)
}

func portAsString(port int) string {
	return strconv.FormatInt(int64(port), 10)
}

func servicePortName(port int) string {
	return "srv-" + portAsString(port)
}

func servicePortAndProtocol(port int) string {
	return join("/", portAsString(port), strings.ToLower(string(servicePortProtocol)))
}
