package typesys

import (
	"maps"
	"sort"
	"strings"
)

// StandardExceptionParents is the shared System exception hierarchy used by
// platform symbols, semantic checks and execution. Callers own the returned map.
func StandardExceptionParents() map[string]string {
	return maps.Clone(standardExceptionParents)
}

// StandardExceptionTypeNames returns the canonical, unqualified System names.
func StandardExceptionTypeNames() []string {
	names := make([]string, 0, len(standardExceptionParents))
	for name := range standardExceptionParents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// StandardExceptionParent accepts unqualified or System-qualified platform names.
// Source declarations resolve their own inheritance before using this catalog.
func StandardExceptionParent(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if prefix, rest, qualified := strings.Cut(name, "."); qualified {
		if !strings.EqualFold(prefix, "System") {
			return "", false
		}
		name = rest
	}
	for known, parent := range standardExceptionParents {
		if strings.EqualFold(name, known) {
			return parent, true
		}
	}
	return "", false
}

var standardExceptionParents = map[string]string{
	"AssertException":                 "Exception",
	"AsyncException":                  "Exception",
	"AuraHandledException":            "Exception",
	"BigObjectException":              "Exception",
	"CalloutException":                "Exception",
	"CanvasException":                 "Exception",
	"DataWeaveScriptException":        "Exception",
	"DmlException":                    "Exception",
	"DuplicateMessageException":       "Exception",
	"EmailException":                  "Exception",
	"EmailTemplateRenderException":    "Exception",
	"EventObjectException":            "Exception",
	"Exception":                       "Object",
	"ExternalObjectException":         "Exception",
	"FatalCursorException":            "Exception",
	"FinalException":                  "Exception",
	"FlowException":                   "Exception",
	"FormulaEvaluationException":      "Exception",
	"FormulaValidationException":      "Exception",
	"HandledException":                "Exception",
	"IllegalArgumentException":        "Exception",
	"InvalidHeaderException":          "Exception",
	"InvalidParameterValueException":  "Exception",
	"InvalidReadOnlyUserDmlException": "Exception",
	"JSONException":                   "Exception",
	"LicenseException":                "Exception",
	"LimitException":                  "Exception",
	"ListException":                   "Exception",
	"MathException":                   "Exception",
	"NoAccessException":               "Exception",
	"NoDataFoundException":            "Exception",
	"NoSuchElementException":          "Exception",
	"NullPointerException":            "Exception",
	"PlatformCacheException":          "Exception",
	"PolyglotException":               "Exception",
	"ProcedureException":              "Exception",
	"QueryException":                  "Exception",
	"RequiredFeatureMissingException": "Exception",
	"SObjectException":                "Exception",
	"SearchException":                 "Exception",
	"SecurityException":               "Exception",
	"SerializationException":          "Exception",
	"StringException":                 "Exception",
	"TouchHandledException":           "Exception",
	"TransientCursorException":        "Exception",
	"TypeException":                   "Exception",
	"UnexpectedException":             "Exception",
	"UnsupportedOperationException":   "Exception",
	"VisualforceException":            "Exception",
	"WaveTemplateException":           "Exception",
	"XmlException":                    "Exception",
}
