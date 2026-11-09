package automation

import (
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestR6FlowMissingUpdateFieldNative(t *testing.T) {
	dir := filepath.Join("testdata", "r6flow_missing_field")
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(strings.Join(row[:2], "/"), func(t *testing.T) {
			root := t.TempDir()
			p := projectWithAccountTier(t, root, true)
			opportunityField := filepath.Join(root, "force-app/main/default/objects/Opportunity/fields/Tier__c.field-meta.xml")
			writeWorkflowTestFile(t, opportunityField, accountTierField)
			p.FieldFiles = append(p.FieldFiles, opportunityField)
			opportunityObject := filepath.Join(root, "force-app/main/default/objects/Opportunity/Opportunity.object-meta.xml")
			writeWorkflowTestFile(t, opportunityObject, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Opportunity</label><pluralLabel>Opportunities</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
			p.ObjectFiles = append(p.ObjectFiles, opportunityObject)
			snapshot := []byte(`{"objects":[{"name":"Opportunity","fieldsComplete":true,"fields":[{"name":"Description","type":"Text"}]}]}`)
			snapshotPath := filepath.Join(root, "snapshot.json")
			if err := os.WriteFile(snapshotPath, snapshot, 0600); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(snapshot)
			p.SchemaSnapshot, p.SchemaSnapshotSHA256 = snapshotPath, fmt.Sprintf("%x", sum)
			src, err := os.ReadFile(filepath.Join(dir, strings.TrimSuffix(row[0], ".0")+"_"+row[1]+".flow-meta.xml"))
			if err != nil {
				t.Fatal(err)
			}
			flow := filepath.Join(root, "force-app/main/default/flows/R6.flow-meta.xml")
			writeWorkflowTestFile(t, flow, string(src))
			p.FlowFiles = []string{flow}
			idx, err := LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			rejected := false
			for _, d := range idx.Diagnostics {
				if d.Code == "GLADEAUTO004" {
					rejected = true
				}
			}
			if rejected != (row[1] == "missing") {
				t.Fatalf("native=%s diagnostics=%v", row[2], idx.Diagnostics)
			}
		})
	}
}

// An undeclared custom field is not proof of absence when the project supplies no schema for the object.
func TestR6FlowUnknownSchemaRetainsUpdateFlow(t *testing.T) {
	root := t.TempDir()
	p := projectWithAccountTier(t, root, true)
	src, err := os.ReadFile(filepath.Join("testdata", "r6flow_missing_field", "62_missing.flow-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	flow := filepath.Join(root, "force-app/main/default/flows/R6.flow-meta.xml")
	writeWorkflowTestFile(t, flow, string(src))
	p.FlowFiles = []string{flow}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range idx.Diagnostics {
		if d.Code == "GLADEAUTO004" {
			t.Fatalf("unknown schema was rejected: %v", d)
		}
	}
}

func TestR6FlowPartialObjectOverlayRetainsUpdateFlow(t *testing.T) {
	root := t.TempDir()
	p := projectWithAccountTier(t, root, true)
	object := filepath.Join(root, "force-app/main/default/objects/Opportunity/Opportunity.object-meta.xml")
	writeWorkflowTestFile(t, object, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Opportunity</label><pluralLabel>Opportunities</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	p.ObjectFiles = append(p.ObjectFiles, object)
	src, err := os.ReadFile(filepath.Join("testdata", "r6flow_missing_field", "62_missing.flow-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	flow := filepath.Join(root, "force-app/main/default/flows/R6.flow-meta.xml")
	writeWorkflowTestFile(t, flow, string(src))
	p.FlowFiles = []string{flow}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range idx.Diagnostics {
		if d.Code == "GLADEAUTO004" {
			t.Fatalf("partial overlay was rejected: %v", d)
		}
	}
}
