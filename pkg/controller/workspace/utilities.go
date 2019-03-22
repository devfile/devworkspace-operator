package workspace

import (
	"strings"
)

func join(sep string, parts ...string) string {
	return strings.Join(parts, sep)
}
