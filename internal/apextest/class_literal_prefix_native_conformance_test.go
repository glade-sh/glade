package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

func TestClassLiteralPrefixNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "class_literal_prefix", "LiteralShapesTest")
}

func TestClassLiteralPrefixNativeRejections(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "class_literal_prefix")
	f, err := os.Open(filepath.Join(dir, "rejections.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 25 || strings.Join(rows[0], ",") != "source_api,route,case,message" {
		t.Fatal("incomplete rejection matrix")
	}
	helper, err := os.ReadFile(filepath.Join(dir, "project", "force-app", "main", "default", "classes", "LiteralShapes.cls"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		key := strings.Join(row[:3], "/")
		if seen[key] || (row[0] != "62.0" && row[0] != "67.0" && row[0] != "68.0") || (row[1] != "anonymous" && row[1] != "isTest") || !strings.Contains(row[3], "Invalid type:") {
			t.Fatalf("invalid native rejection %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join(dir, "rejections", row[2]+".apex"))
			if err != nil {
				t.Fatal(err)
			}
			classes := map[string]string{"LiteralShapes": string(helper)}
			if row[1] == "isTest" {
				named, err := os.ReadFile(filepath.Join(dir, "rejections", row[2]+".cls"))
				if err != nil {
					t.Fatal(err)
				}
				classes[strings.Fields(string(named))[3]] = string(named)
			}
			index := nativeBoundaryIndex(t, row[0], classes, nil)
			var diagnostics []diagnostic.Diagnostic
			if row[1] == "anonymous" {
				diagnostics = sema.AnalyzeAnonymous(index, string(body), row[0]).Diagnostics
			} else {
				for _, item := range sema.Analyze(index).Diagnostics {
					if strings.HasPrefix(filepath.Base(item.File), "LiteralReject") {
						diagnostics = append(diagnostics, item)
					}
				}
			}
			for _, item := range diagnostics {
				literal := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(string(body), "Type rejected = ")), ";")
				typename := strings.TrimSuffix(literal, ".class")
				leaf := typename[strings.LastIndex(typename, ".")+1:] + ".class"
				message := strings.ToLower(item.NativeMessage + " " + item.Message)
				if item.Severity == diagnostic.Error && (strings.Contains(message, strings.ToLower(typename)) || strings.Contains(message, strings.ToLower(leaf))) {
					return
				}
			}
			t.Fatalf("native rejects %s; Glade accepted", row[3])
		})
	}
}
