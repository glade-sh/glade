package apexversion

import (
	"strconv"
	"strings"
)

type Feature uint8

const (
	LegacySiteURLHelpers Feature = iota
	LegacyCacheValueSize
	LegacyCacheValidateKeys
	SecureDefaults
)

func Major(raw string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ".")
	if len(parts) == 0 || parts[0] == "" {
		return 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 1 {
		return 0, false
	}
	if len(parts) > 2 {
		return 0, false
	}
	if len(parts) == 2 {
		if parts[1] == "" {
			return 0, false
		}
		if _, err := strconv.Atoi(parts[1]); err != nil {
			return 0, false
		}
	}
	return major, true
}

func Enabled(raw string, feature Feature) bool {
	major, ok := Major(raw)
	if !ok {
		return false
	}
	switch feature {
	case LegacySiteURLHelpers:
		return major <= 29
	case LegacyCacheValueSize:
		return major <= 49
	case LegacyCacheValidateKeys:
		return major <= 54
	case SecureDefaults:
		return major >= 67
	default:
		return false
	}
}
