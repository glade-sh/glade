package trace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	FormatChromeTraceEvent = "chrome-trace-event"
	Version                = 1
)

const (
	ArgEntryPoint    = "entryPoint"
	ArgClass         = "class"
	ArgMethod        = "method"
	ArgOperation     = "operation"
	ArgOperationID   = "operationId"
	ArgObject        = "object"
	ArgObjects       = "objects"
	ArgField         = "field"
	ArgQuery         = "query"
	ArgQueryHash     = "queryHash"
	ArgRows          = "rows"
	ArgFile          = "file"
	ArgLine          = "line"
	ArgColumn        = "column"
	ArgSourceOffset  = "sourceOffset"
	ArgNamespace     = "namespace"
	ArgTransactionID = "transactionId"
)

type Phase string

const (
	PhaseInstant  Phase = "i"
	PhaseComplete Phase = "X"
)

type Event struct {
	Name      string         `json:"name"`
	Category  string         `json:"cat,omitempty"`
	Phase     Phase          `json:"ph"`
	Timestamp int64          `json:"ts"`
	Duration  int64          `json:"dur,omitempty"`
	ProcessID int            `json:"pid"`
	ThreadID  int            `json:"tid"`
	Scope     string         `json:"s,omitempty"`
	Args      map[string]any `json:"args,omitempty"`
}

type Document struct {
	Format      string         `json:"format"`
	Version     int            `json:"version"`
	TraceEvents []Event        `json:"traceEvents"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

func Instant(name, category string, timestamp int64, args map[string]any) Event {
	return Event{
		Name:      name,
		Category:  category,
		Phase:     PhaseInstant,
		Timestamp: timestamp,
		ProcessID: 1,
		ThreadID:  1,
		Scope:     "t",
		Args:      args,
	}
}

func Duration(name, category string, timestamp, duration int64, args map[string]any) Event {
	return Event{
		Name:      name,
		Category:  category,
		Phase:     PhaseComplete,
		Timestamp: timestamp,
		Duration:  duration,
		ProcessID: 1,
		ThreadID:  1,
		Args:      args,
	}
}

func NewDocument(events []Event) Document {
	return Document{
		Format:      FormatChromeTraceEvent,
		Version:     Version,
		TraceEvents: events,
	}
}

func SourceArgs(file string, line, column int) map[string]any {
	return AddSourceArgs(nil, file, line, column)
}

func AddSourceArgs(args map[string]any, file string, line, column int) map[string]any {
	if args == nil {
		args = make(map[string]any)
	}
	file = strings.TrimSpace(file)
	if file != "" {
		args[ArgFile] = filepath.ToSlash(file)
	}
	if line > 0 {
		args[ArgLine] = line
	}
	if column > 0 {
		args[ArgColumn] = column
	}
	return args
}

func StableOperationID(file string, line int, operation, detail string) string {
	parts := []string{
		filepath.ToSlash(strings.TrimSpace(file)),
		strconv.Itoa(line),
		NormalizeFactText(operation),
		NormalizeFactText(detail),
	}
	return sha256Hex(strings.Join(parts, "\x00"))
}

func StableQueryHash(query string) string {
	return sha256Hex(NormalizeQueryText(query))
}

func NormalizeFactText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func NormalizeQueryText(value string) string {
	var out strings.Builder
	pendingSpace := false
	inLiteral := false
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if inLiteral {
			out.WriteByte(ch)
			if ch == '\'' {
				if i+1 < len(value) && value[i+1] == '\'' {
					i++
					out.WriteByte(value[i])
					continue
				}
				inLiteral = false
			}
			continue
		}
		if ch == '\'' {
			if pendingSpace && out.Len() > 0 {
				out.WriteByte(' ')
			}
			pendingSpace = false
			inLiteral = true
			out.WriteByte(ch)
			continue
		}
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '\f' || ch == '\v' {
			pendingSpace = true
			continue
		}
		if pendingSpace && out.Len() > 0 {
			out.WriteByte(' ')
		}
		pendingSpace = false
		out.WriteByte(ch)
	}
	return strings.TrimSpace(out.String())
}

func WriteJSON(w io.Writer, doc Document) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
