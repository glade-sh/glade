package cliui

import "strings"

func ColorEnabled(isTTY bool, noColor string) bool {
	return isTTY && strings.TrimSpace(noColor) == ""
}

func Row(status, name, value string) string {
	return status + " " + name + "  " + value
}
