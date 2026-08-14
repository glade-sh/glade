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
	major, ok := unsignedComponent(parts[0])
	if !ok || major < 1 {
		return 0, false
	}
	if len(parts) > 2 {
		return 0, false
	}
	if len(parts) == 2 {
		if parts[1] == "" {
			return 0, false
		}
		if _, ok := unsignedComponent(parts[1]); !ok {
			return 0, false
		}
	}
	return major, true
}

func unsignedComponent(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	for _, char := range raw {
		if char < '0' || char > '9' {
			return 0, false
		}
	}
	value, err := strconv.Atoi(raw)
	return value, err == nil
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
