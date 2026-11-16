package vm

import (
	"fmt"
	"regexp"
	"strings"
)

// soqlSetOptionsClause matches the trailing SET OPTIONS bind that the API 68
// capture runs after LIMIT (api68_native queryoptions QO-NULL-*,
// QO-SETOPTIONS-*). Quoted text cannot end in a bare bind, so a match is
// always the clause itself.
var soqlSetOptionsClause = regexp.MustCompile(`(?is)\s+SET\s+OPTIONS\s*:\s*([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\s*$`)

const soqlSetOptionsNullMessage = "SET OPTIONS bind value cannot be null"

// applySOQLSetOptions resolves the SET OPTIONS bind and removes the clause.
// A null bind raises the captured QueryException. The captured options
// (a dataspace and honorEmptyStrings on a platform object) leave the platform
// query unchanged; other bound values were not captured.
func (vm *VM) applySOQLSetOptions(raw string, lookup func(string) (Value, error)) (string, error) {
	if !containsFoldASCII(raw, "options") {
		return raw, nil
	}
	match := soqlSetOptionsClause.FindStringSubmatchIndex(raw)
	if match == nil {
		return raw, nil
	}
	value, err := lookup(raw[match[2]:match[3]])
	if err != nil {
		return "", err
	}
	if value.Kind == ValueNull {
		return "", newExceptionError("QueryException", soqlSetOptionsNullMessage)
	}
	if value.Kind != ValueObject || value.Type != queryOptionsType {
		return "", unsupportedCallError("SET OPTIONS bind of " + value.Type)
	}
	return raw[:match[0]], nil
}

// containsFoldASCII is an allocation-free ASCII case-insensitive Contains for
// the per-query SET OPTIONS precheck. needle must be lower case.
func containsFoldASCII(text, needle string) bool {
	for i := 0; i+len(needle) <= len(text); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			c := text[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// Database.QueryOptions and its builder (api68_native queryoptions
// QO-BUILDER-*, QO-SETOPTIONS-*, source APIs 67.0 and 68.0).
const (
	queryOptionsType        = "Database.QueryOptions"
	queryOptionsBuilderType = "Database.QueryOptionsBuilder"
)

var queryOptionsFields = []string{"dataspace", "explicitNamespace", "honorEmptyStrings"}

func newQueryOptionsBuilder() Value {
	builder := Object(queryOptionsBuilderType)
	for _, field := range queryOptionsFields {
		builder.Fields[field] = Null
	}
	return builder
}

// callQueryOptionsMember handles builder setters, which update the builder in
// place and return it, build(), which snapshots the current values, and the
// QueryOptions getters.
func callQueryOptionsMember(receiver Value, method string, args []Value) (Value, bool, bool, error) {
	method = strings.ToLower(method)
	if receiver.Type == queryOptionsType {
		if len(args) != 0 {
			return Null, false, false, nil
		}
		switch method {
		case "getdataspace":
			return receiver.Fields["dataspace"], false, true, nil
		case "getexplicitnamespace":
			return receiver.Fields["explicitNamespace"], false, true, nil
		case "gethonoremptystrings":
			return receiver.Fields["honorEmptyStrings"], false, true, nil
		}
		return Null, false, false, nil
	}
	switch method {
	case "build":
		if len(args) != 0 {
			return Null, false, false, nil
		}
		options := Object(queryOptionsType)
		for _, field := range queryOptionsFields {
			options.Fields[field] = receiver.Fields[field]
		}
		return options, false, true, nil
	case "withdataspace":
		if len(args) != 1 {
			return Null, false, false, nil
		}
		if args[0].Kind == ValueNull || args[0].Kind == ValueString && args[0].Text == "" {
			return Null, false, true, newExceptionError("System.IllegalArgumentException", "dataspace cannot be null or empty")
		}
		receiver.Fields["dataspace"] = args[0]
		return receiver, true, true, nil
	case "withexplicitnamespace":
		if len(args) != 1 {
			return Null, false, false, nil
		}
		if args[0].Kind == ValueNull {
			return Null, false, true, newExceptionError("System.IllegalArgumentException", "explicitNamespace cannot be null")
		}
		receiver.Fields["explicitNamespace"] = args[0]
		return receiver, true, true, nil
	case "withhonoremptystrings":
		if len(args) != 1 {
			return Null, false, false, nil
		}
		if args[0].Kind == ValueNull {
			// A null honorEmptyStrings was not captured.
			return Null, false, true, unsupportedCallError(fmt.Sprintf("%s.withHonorEmptyStrings(null)", queryOptionsBuilderType))
		}
		receiver.Fields["honorEmptyStrings"] = args[0]
		return receiver, true, true, nil
	}
	return Null, false, false, nil
}
