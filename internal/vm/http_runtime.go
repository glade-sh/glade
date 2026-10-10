package vm

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// R008/R072/R083 and H045-H052 share the same DTO display through explicit
// toString, String.valueOf, and String concatenation.
func httpDTOString(receiver Value) (string, bool) {
	// The resolved platform construction path stamps this ownership. A source
	// class with the same name uses the ordinary project-class field display.
	if !receiver.platformHTTPDTO {
		return "", false
	}
	switch receiver.Type {
	case "HttpRequest":
		return "System.HttpRequest[Endpoint=" + receiver.Fields["endpoint"].String() + ", Method=" + receiver.Fields["method"].String() + "]", true
	case "HttpResponse":
		return "System.HttpResponse[Status=" + receiver.Fields["status"].String() + ", StatusCode=" + receiver.Fields["statusCode"].String() + "]", true
	default:
		return "", false
	}
}

// HTTP DTOs distinguish an unset body from an explicitly empty body (R003/R004,
// R040/R041, R079/R080 and R103). Invalid bytes are replaced only when reading
// the HTTP String body; Blob.toString retains its own UTF-8 validation (R172).
func httpBodyString(receiver Value) Value {
	body := receiver.Fields["body"]
	if body.Kind != ValueString {
		return String("")
	}
	return String(httpDecodeBodyText(body.Text))
}

// H028-H040 distinguish malformed UTF-8 prefixes from separate invalid bytes:
// ff ff becomes two replacements, but an incomplete e2 80 prefix and a complete
// encoded surrogate each become one. This decoder is confined to HTTP bodies.
func httpDecodeBodyText(text string) string {
	if utf8.ValidString(text) {
		return text
	}
	var out strings.Builder
	out.Grow(len(text))
	for len(text) > 0 {
		r, size := utf8.DecodeRuneInString(text)
		if r != utf8.RuneError || size != 1 {
			out.WriteString(text[:size])
			text = text[size:]
			continue
		}
		width := 1
		switch {
		case text[0] >= 0xc2 && text[0] <= 0xdf:
			width = 2
		case text[0] >= 0xe0 && text[0] <= 0xef:
			width = 3
		case text[0] >= 0xf0 && text[0] <= 0xf4:
			width = 4
		}
		if width > 1 && len(text) > 1 && text[1] >= 0x80 && text[1] <= 0xbf &&
			(text[0] != 0xe0 || text[1] >= 0xa0) &&
			(text[0] != 0xf0 || text[1] >= 0x90) &&
			(text[0] != 0xf4 || text[1] <= 0x8f) {
			size = 2
			for size < width && size < len(text) && text[size] >= 0x80 && text[size] <= 0xbf {
				size++
			}
		}
		out.WriteRune(utf8.RuneError)
		text = text[size:]
	}
	return out.String()
}

func httpBodyBlob(receiver Value) Value {
	body := receiver.Fields["body"]
	if body.Kind != ValueString {
		return Null
	}
	return platformScalar("Blob", body.Text)
}

func httpNullArgument(args []Value) error {
	for i, arg := range args {
		if arg.Kind == ValueNull {
			return newExceptionError("NullPointerException", fmt.Sprintf("Argument %d cannot be null", i+1))
		}
	}
	return nil
}

// Request header names use HTTP tokens (R061/R063). Response and REST header
// maps have separate contracts and do not use this validation.
func validHTTPRequestHeader(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || strings.ContainsRune("!#$%&'*+-.^_`|~", c) {
			continue
		}
		return false
	}
	return true
}

func restAddHeader(receiver Value, args []Value) (Value, Value, bool, bool, error) {
	if len(args) != 2 || (args[0].Kind != ValueString && args[0].Kind != ValueNull) || (args[1].Kind != ValueString && args[1].Kind != ValueNull) {
		return Null, receiver, false, true, fmt.Errorf("%s.addHeader expects name and value Strings", receiver.Type)
	}
	if args[0].Kind == ValueNull {
		return Null, receiver, false, true, newExceptionError("InvalidHeaderException", "Header name \"null\" is not allowed.")
	}
	restMapPut(&receiver, "headers", args[0].Text, args[1], false)
	return Null, receiver, true, true, nil
}
