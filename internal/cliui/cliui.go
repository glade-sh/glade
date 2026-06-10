package cliui

import "strings"

func ColorEnabled(isTTY bool, noColor string) bool {
	return isTTY && strings.TrimSpace(noColor) == ""
}
