package debuglog

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func WriteText(w io.Writer, annotated AnnotatedLog, minConfidence float64) error {
	for _, entry := range annotated.Entries {
		if _, err := fmt.Fprintln(w, entry.Entry.Raw); err != nil {
			return err
		}
		if entry.Best.Confidence >= minConfidence {
			parts := make([]string, 0, 3)
			if entry.Best.File != "" {
				if entry.Best.Line > 0 {
					parts = append(parts, entry.Best.File+":"+itoa(entry.Best.Line))
				} else {
					parts = append(parts, entry.Best.File)
				}
			}
			if entry.Best.Symbol != "" {
				parts = append(parts, entry.Best.Symbol)
			}
			parts = append(parts,
				fmt.Sprintf("confidence=%.2f", entry.Best.Confidence),
				"reason="+entry.Best.Reason,
			)
			_, err := fmt.Fprintf(w, "  => %s\n", strings.Join(parts, " "))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func WriteJSON(w io.Writer, annotated AnnotatedLog) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(annotated)
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
