package apexversion

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

type Range struct {
	Since int `json:"since,omitempty"`
	Until int `json:"until,omitempty"`
}

func (r Range) Allows(raw string) bool {
	major, ok := Major(raw)
	if !ok || major < r.Since {
		return false
	}
	return r.Until == 0 || major < r.Until
}

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

func ResolveSource(raw string) (string, error) {
	requested := strings.TrimSpace(raw)
	normalized, err := PreserveSource(requested)
	if err != nil {
		return "", unsupportedSourceVersion(requested)
	}
	if !slices.Contains(SupportedSourceAPIVersions, normalized) {
		return "", unsupportedSourceVersion(requested)
	}
	return normalized, nil
}

// PreserveSource canonicalizes a positive whole Apex source version without
// moving historical versions into the checked Salesforce parity window.
func PreserveSource(raw string) (string, error) {
	requested := strings.TrimSpace(raw)
	if requested == "" {
		requested = DefaultSourceAPIVersion
	}
	if len(SupportedSourceAPIVersions) == 0 {
		return "", unsupportedHistoricalSourceVersion(requested)
	}
	numeric := strings.TrimPrefix(requested, "v")
	parts := strings.Split(numeric, ".")
	if len(parts) != 2 {
		return "", unsupportedHistoricalSourceVersion(requested)
	}
	major, majorOK := unsignedComponent(parts[0])
	minor, minorOK := unsignedComponent(parts[1])
	if !majorOK || !minorOK || major < 1 || minor != 0 {
		return "", unsupportedHistoricalSourceVersion(requested)
	}
	normalized := fmt.Sprintf("%d.0", major)
	latest := SupportedSourceAPIVersions[len(SupportedSourceAPIVersions)-1]
	latestMajor, _ := Major(latest)
	if major > latestMajor {
		return "", unsupportedHistoricalSourceVersion(requested)
	}
	return normalized, nil
}

func unsupportedSourceVersion(requested string) error {
	return fmt.Errorf("unsupported source API version %q; supported versions: %s", requested, strings.Join(SupportedSourceAPIVersions, ", "))
}

func unsupportedHistoricalSourceVersion(requested string) error {
	if len(SupportedSourceAPIVersions) == 0 {
		return fmt.Errorf("unsupported source API version %q for Apex; no checked source API versions configured", requested)
	}
	return fmt.Errorf("unsupported source API version %q for Apex; expected a whole version no later than %s", requested, SupportedSourceAPIVersions[len(SupportedSourceAPIVersions)-1])
}

func AtLeast(raw string, minimum int) bool {
	major, ok := Major(raw)
	return ok && major >= minimum
}

func Before(raw string, minimum int) bool {
	major, ok := Major(raw)
	return ok && major < minimum
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
