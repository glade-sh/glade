package vm

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"sync/atomic"
)

// jsonDecimalNumberText writes a Double or Decimal the way Salesforce does:
// the same text as String.valueOf. A Double keeps its fractional marker (5.0)
// and uses exponent notation outside [1e-3, 1e7) (1.0E20, 1.0E-4). A Decimal
// keeps its scale and uses exponent notation below 1E-6. A non-finite Double
// is written bare (Infinity, -Infinity, NaN), as the platform does.
func jsonDecimalNumberText(value Value) (string, bool) {
	if value.Kind != ValueDecimal {
		return "", false
	}
	if isFloatBackedDecimal(value) {
		return doubleDisplayText(value.Decimal), true
	}
	if math.IsNaN(value.Decimal) || math.IsInf(value.Decimal, 0) {
		return "", false
	}
	text := value.Text
	if text == "" || strings.TrimSpace(text) == "" {
		return "", false
	}
	// Only a magnitude below 1E-6 changes spelling; plain text with fewer than
	// six zeros after the point (or an exponent already) is final as stored.
	if strings.IndexByte(text, 'E') >= 0 || strings.IndexByte(text, 'e') >= 0 || !strings.Contains(text, ".000000") {
		return text, true
	}
	return numericDisplayText(value), true
}

// jsonNonFiniteNumber is a Double that encoding/json cannot write as a number.
// It encodes as a marked string; jsonRestoreNonFiniteNumbers then strips the
// quotes so the output carries the bare token. The restore scan only runs
// after a non-finite value has been encoded in this process.
type jsonNonFiniteNumber string

const jsonNonFiniteMarker = `\u0000glade-nonfinite:`

var jsonNonFiniteEncoded atomic.Bool

func (number jsonNonFiniteNumber) MarshalJSON() ([]byte, error) {
	jsonNonFiniteEncoded.Store(true)
	return []byte(`"` + jsonNonFiniteMarker + string(number) + `"`), nil
}

func jsonRestoreNonFiniteNumbers(data []byte) []byte {
	if !jsonNonFiniteEncoded.Load() {
		return data
	}
	marker := []byte(jsonNonFiniteMarker)
	for {
		at := bytes.Index(data, marker)
		if at < 1 || data[at-1] != '"' {
			return data
		}
		end := bytes.IndexByte(data[at+len(marker):], '"')
		if end < 0 {
			return data
		}
		end += at + len(marker)
		restored := make([]byte, 0, len(data))
		restored = append(restored, data[:at-1]...)
		restored = append(restored, data[at+len(marker):end]...)
		restored = append(restored, data[end+1:]...)
		data = restored
	}
}

// jsonDecimalNumberValue is the encodable form of a Double or Decimal.
func jsonDecimalNumberValue(value Value) (any, bool) {
	text, ok := jsonDecimalNumberText(value)
	if !ok {
		return nil, false
	}
	if math.IsNaN(value.Decimal) || math.IsInf(value.Decimal, 0) {
		return jsonNonFiniteNumber(text), true
	}
	return json.Number(text), true
}
