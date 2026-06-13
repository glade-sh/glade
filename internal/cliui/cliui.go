package cliui

import (
	"os"
	"strings"
)

func ColorEnabled(isTTY bool, noColor string) bool {
	return isTTY && strings.TrimSpace(noColor) == "" && !stringsEqualFold(getTerm(), "dumb")
}

var getTerm = func() string { return os.Getenv("TERM") }

var stringsEqualFold = func(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
