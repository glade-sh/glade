package vm

import (
	"errors"
	"testing"
)

func TestCompileSingleQuotedStringLineBreaks(t *testing.T) {
	for _, api := range []string{"59.0", "62.0", "67.0", "68.0"} {
		for _, test := range []struct {
			name   string
			source string
			reject bool
		}{
			{"query", "Object r=Database.query('SELECT Id FROM Account // bind\n');", true},
			{"countQuery", "Object r=Database.countQuery('SELECT COUNT() FROM Account // bind\n');", true},
			{"queryWithBinds", "Object r=Database.queryWithBinds('SELECT Id FROM Account // bind\n',new Map<String,Object>(),AccessLevel.USER_MODE);", true},
			{"jsonMultiline", "Object value=JSON.deserialize('{\n  \"Name\":{}}',String.class);", false},
			{"jsonMultilineStrict", "Object value=JSON.deserializeStrict('{\n  \"Name\":{}}',String.class);", false},
			{"jsonMultilineSlashText", "Object value=JSON.deserialize('{\"Text\":\"//\",\n  \"Name\":{}}',String.class);", false},
			{"ordinaryMultiline", "String payload='{\n  \"Name\":{}}';", false},
			{"queryEscapedNewline", `Object r=Database.query('SELECT Id FROM Account // bind\n');`, false},
			{"escapedNewline", `String query='SELECT Id FROM Account // bind\n';`, false},
			{"tripleQuotedNewline", "String query='''\nSELECT Id FROM Account // bind\n''';", false},
		} {
			t.Run(api+"/"+test.name, func(t *testing.T) {
				_, err := CompileAnonymousWithOptions(test.source, CompileOptions{APIVersion: api})
				if !test.reject {
					if err != nil {
						t.Fatal(err)
					}
					return
				}
				var syntax *ApexSyntaxError
				if !errors.As(err, &syntax) || syntax.NativeMessage != "Illegal string literal: Line breaks are not allowed in string literals" {
					t.Fatalf("ordinary quoted newline: %v", err)
				}
			})
		}
	}
}
