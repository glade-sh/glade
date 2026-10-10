package dataweave

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"regexp"
	"strconv"
	"time"
	"unicode/utf8"
)

// TypedValue carries an explicitly identified Apex value without floating-point
// conversion. Type retains a declared collection type or concrete object name.
// The Apex host owns validation/coercion against its schema and declared types.
type TypedValue struct {
	Kind     string
	Type     string
	Text     string
	Boolean  bool
	Elements []TypedValue
	Fields   []TypedField
}
type TypedField struct {
	Name  string
	Value TypedValue
}

const maxTypedNodes = 100000

var typedDecimalPattern = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)
var typedKindCodes = map[string]byte{"null": 0, "string": 1, "integer": 2, "decimal": 3, "boolean": 4, "list": 5, "map": 6, "object": 7, "long": 8, "date": 9, "datetime": 10, "time": 11, "blob": 12, "query-list": 13}
var typedKinds = []string{"null", "string", "integer", "decimal", "boolean", "list", "map", "object", "long", "date", "datetime", "time", "blob", "query-list"}

func encodeTyped(value TypedValue) ([]byte, error) {
	var out bytes.Buffer
	nodes := 0
	var write func(TypedValue, int) error
	write = func(value TypedValue, depth int) error {
		nodes++
		if depth > 64 || nodes > maxTypedNodes {
			return fmt.Errorf("typed DataWeave value exceeds structure limit")
		}
		code, ok := typedKindCodes[value.Kind]
		if !ok {
			return fmt.Errorf("unsupported DataWeave typed kind %q", value.Kind)
		}
		if !utf8.ValidString(value.Type) || !utf8.ValidString(value.Text) {
			return fmt.Errorf("typed DataWeave value must contain valid UTF8")
		}
		if value.Kind != "list" && value.Kind != "query-list" && len(value.Elements) > 0 {
			return fmt.Errorf("typed DataWeave elements require list kind")
		}
		if value.Kind != "map" && value.Kind != "object" && len(value.Fields) > 0 {
			return fmt.Errorf("typed DataWeave fields require map or object kind")
		}
		out.WriteByte(code)
		if err := writeField(&out, []byte(value.Type)); err != nil {
			return err
		}
		switch value.Kind {
		case "string", "integer", "decimal", "long", "date", "datetime", "time", "blob":
			if value.Kind == "integer" || value.Kind == "long" {
				bits := 32
				if value.Kind == "long" {
					bits = 64
				}
				if _, err := strconv.ParseInt(value.Text, 10, bits); err != nil {
					return fmt.Errorf("invalid typed Apex %s: %w", value.Kind, err)
				}
			}
			if value.Kind == "decimal" && !typedDecimalPattern.MatchString(value.Text) {
				return fmt.Errorf("invalid typed Apex Decimal")
			}
			switch value.Kind {
			case "date", "datetime", "time":
				layout := map[string]string{"date": "2006-01-02", "datetime": time.RFC3339Nano, "time": "15:04:05.999999999"}[value.Kind]
				if _, err := time.Parse(layout, value.Text); err != nil {
					return fmt.Errorf("invalid typed Apex %s: %w", value.Kind, err)
				}
			case "blob":
				if _, err := base64.StdEncoding.Strict().DecodeString(value.Text); err != nil {
					return fmt.Errorf("invalid typed Apex Blob: %w", err)
				}
			}
			if err := writeField(&out, []byte(value.Text)); err != nil {
				return err
			}
		case "boolean":
			if value.Boolean {
				out.WriteByte(1)
			} else {
				out.WriteByte(0)
			}
		case "list", "query-list":
			if len(value.Elements) > maxTypedNodes {
				return fmt.Errorf("too many typed DataWeave elements")
			}
			binary.Write(&out, binary.BigEndian, uint32(len(value.Elements)))
			for _, element := range value.Elements {
				if err := write(element, depth+1); err != nil {
					return err
				}
			}
		case "map", "object":
			if value.Kind == "object" && value.Type == "" {
				return fmt.Errorf("typed DataWeave object requires type")
			}
			if len(value.Fields) > maxTypedNodes {
				return fmt.Errorf("too many typed DataWeave fields")
			}
			binary.Write(&out, binary.BigEndian, uint32(len(value.Fields)))
			seen := map[string]bool{}
			for _, field := range value.Fields {
				if !utf8.ValidString(field.Name) || seen[field.Name] {
					return fmt.Errorf("invalid or duplicate typed DataWeave field")
				}
				seen[field.Name] = true
				if err := writeField(&out, []byte(field.Name)); err != nil {
					return err
				}
				if err := write(field.Value, depth+1); err != nil {
					return err
				}
			}
		}
		if out.Len() > maxFieldBytes {
			return fmt.Errorf("typed DataWeave value exceeds byte limit")
		}
		return nil
	}
	if err := write(value, 0); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func decodeTyped(data []byte) (TypedValue, error) {
	in := bytes.NewReader(data)
	nodes := 0
	field := func() (string, error) {
		v, err := readField(in)
		if err != nil {
			return "", err
		}
		if !utf8.Valid(v) {
			return "", fmt.Errorf("invalid typed output UTF8")
		}
		return string(v), nil
	}
	count := func() (int, error) {
		var n uint32
		if err := binary.Read(in, binary.BigEndian, &n); err != nil {
			return 0, err
		}
		if n > maxTypedNodes || n > uint32(in.Len()) {
			return 0, fmt.Errorf("invalid typed output count")
		}
		return int(n), nil
	}
	var read func(int) (TypedValue, error)
	read = func(depth int) (TypedValue, error) {
		var value TypedValue
		nodes++
		if depth > 64 || nodes > maxTypedNodes {
			return value, fmt.Errorf("typed output structure limit")
		}
		code, err := in.ReadByte()
		if err != nil {
			return value, err
		}
		if int(code) >= len(typedKinds) {
			return value, fmt.Errorf("invalid typed output kind")
		}
		value.Kind = typedKinds[code]
		if value.Type, err = field(); err != nil {
			return value, err
		}
		switch value.Kind {
		case "string", "integer", "decimal", "long", "date", "datetime", "time", "blob":
			value.Text, err = field()
		case "boolean":
			var b byte
			b, err = in.ReadByte()
			if b > 1 {
				return value, fmt.Errorf("invalid typed Boolean")
			}
			value.Boolean = b == 1
		case "list", "query-list":
			var n int
			n, err = count()
			if err != nil {
				return value, err
			}
			value.Elements = make([]TypedValue, n)
			for i := range value.Elements {
				if value.Elements[i], err = read(depth + 1); err != nil {
					return value, err
				}
			}
		case "map", "object":
			var n int
			n, err = count()
			if err != nil {
				return value, err
			}
			value.Fields = make([]TypedField, n)
			for i := range value.Fields {
				if value.Fields[i].Name, err = field(); err != nil {
					return value, err
				}
				if value.Fields[i].Value, err = read(depth + 1); err != nil {
					return value, err
				}
			}
		}
		return value, err
	}
	value, err := read(0)
	if err != nil {
		return value, err
	}
	if in.Len() != 0 {
		return value, fmt.Errorf("trailing typed output bytes")
	}
	// Apply the same kind/value/duplicate/structure constraints to engine output.
	if _, err := encodeTyped(value); err != nil {
		return value, err
	}
	return value, nil
}
