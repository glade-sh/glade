package vm

import (
	"fmt"
	"strings"
	"time"
)

// Owned named JSON observations at source APIs 62/63/67 accept Y-MM-DD and
// YY-MM-DD with the same serialization as their zero-padded year. Keep this
// normalization specific to JSON; Date.valueOf and calendar accessors retain
// their separate behavior.
func parseJSONDateText(text string) (time.Time, error) {
	parts := strings.Split(strings.TrimSpace(text), "-")
	if len(parts) != 3 || !isAllDigits(parts[0]) || !isAllDigits(parts[1]) || !isAllDigits(parts[2]) {
		return time.Time{}, fmt.Errorf("unsupported Date value %q", text)
	}
	value, err := parseDateText(text)
	if err == nil {
		return value, nil
	}
	year, rest, ok := strings.Cut(text, "-")
	if !ok || len(year) < 1 || len(year) > 2 || !isAllDigits(year) ||
		len(rest) != 5 || rest[2] != '-' || !isAllDigits(rest[:2]) || !isAllDigits(rest[3:]) {
		return value, err
	}
	return parseDateText(strings.Repeat("0", 4-len(year)) + year + "-" + rest)
}

func jsonDateDeserializeErrorMessage(text string) string {
	if separator := strings.IndexByte(text, ' '); separator > 0 {
		if _, err := parseDateText(text[:separator]); err == nil {
			return fmt.Sprintf("Invalid format: %q is malformed at %q", text, text[separator:])
		}
	}
	return fmt.Sprintf("JSON.deserialize cannot parse Date %q", text)
}
