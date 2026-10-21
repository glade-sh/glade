package schema

import (
	"maps"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestLoadProjectPreservesSharingReasonAuthoredLabel(t *testing.T) {
	root := t.TempDir()
	objectPath := filepath.Join(root, "force-app/main/default/objects/AuditArchive__c/AuditArchive__c.object-meta.xml")
	writeFile(t, objectPath, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Audit archive</label><pluralLabel>Audit archives</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(filepath.Dir(objectPath), "sharingReasons/Authored_Audit__c.sharingReason-meta.xml"), `<SharingReason xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Authored_Audit__c</fullName><label>Audit &amp; review</label></SharingReason>`)
	s, err := LoadProject(project.Project{Root: root, ObjectFiles: []string{objectPath}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Objects) != 1 {
		t.Fatalf("loaded objects = %#v", s.Objects)
	}
	object := s.Objects[0]
	if !reflect.DeepEqual(object.SharingReasons, []string{"Authored_Audit__c"}) {
		t.Fatalf("sharing reason tokens changed: %#v", object.SharingReasons)
	}
	if got := object.SharingReasonLabels["Authored_Audit__c"]; got != "Audit & review" {
		t.Fatalf("authored sharing reason label = %q", got)
	}
}

// Older local fixtures can omit the required source label. Loading their token
// must remain possible without inventing an authored label.
func TestLoadProjectSharingReasonWithoutLabelPreservesLegacyToken(t *testing.T) {
	root := t.TempDir()
	objectPath := filepath.Join(root, "force-app/main/default/objects/AuditArchive__c/AuditArchive__c.object-meta.xml")
	writeFile(t, objectPath, `<CustomObject><label>Audit archive</label><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(filepath.Dir(objectPath), "sharingReasons/Legacy_Audit__c.sharingReason-meta.xml"), `<SharingReason><fullName>Legacy_Audit__c</fullName></SharingReason>`)
	s, err := LoadProject(project.Project{Root: root, ObjectFiles: []string{objectPath}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Objects) != 1 || !reflect.DeepEqual(s.Objects[0].SharingReasons, []string{"Legacy_Audit__c"}) {
		t.Fatalf("legacy sharing reason token = %#v", s.Objects)
	}
	if got := s.Objects[0].SharingReasonLabels["Legacy_Audit__c"]; got != "" {
		t.Fatalf("missing source label acquired authored label %q", got)
	}
}

func TestMergeObjectMetadataPreservesSharingReasonLabelsWithoutAliasing(t *testing.T) {
	for _, emptyBase := range []bool{false, true} {
		t.Run(map[bool]string{false: "existing labels", true: "empty labels"}[emptyBase], func(t *testing.T) {
			base := Object{Name: "AuditArchive__c", SharingReasons: []string{"Base_Audit__c"}}
			if !emptyBase {
				base.SharingReasonLabels = map[string]string{"Base_Audit__c": "Base audit"}
			}
			overlay := Object{Name: "AuditArchive__c", SharingReasons: []string{"Authored_Audit__c"}, SharingReasonLabels: map[string]string{"Authored_Audit__c": "Audit & review"}}
			baseLabels, overlayLabels := maps.Clone(base.SharingReasonLabels), maps.Clone(overlay.SharingReasonLabels)
			merged := base
			mergeObjectMetadata(&merged, overlay)
			if !reflect.DeepEqual(merged.SharingReasons, []string{"Base_Audit__c", "Authored_Audit__c"}) {
				t.Fatalf("merged sharing reason tokens = %#v", merged.SharingReasons)
			}
			want := map[string]string{"Authored_Audit__c": "Audit & review"}
			if !emptyBase {
				want["Base_Audit__c"] = "Base audit"
			}
			if !reflect.DeepEqual(merged.SharingReasonLabels, want) {
				t.Fatalf("merged labels = %#v, want %#v", merged.SharingReasonLabels, want)
			}
			merged.SharingReasonLabels["Authored_Audit__c"] = "Changed merged label"
			merged.SharingReasonLabels["Base_Audit__c"] = "Changed merged base label"
			if !reflect.DeepEqual(base.SharingReasonLabels, baseLabels) || !reflect.DeepEqual(overlay.SharingReasonLabels, overlayLabels) {
				t.Fatalf("merged label map aliases input: base=%#v overlay=%#v", base.SharingReasonLabels, overlay.SharingReasonLabels)
			}
		})
	}
}

// Token merging already compares names without case. Label lookup must preserve
// that local merge contract without changing the retained token's spelling.
func TestMergeObjectMetadataSharingReasonLabelCaseControl(t *testing.T) {
	for _, test := range []struct{ name, baseLabel, want string }{
		{"existing label wins", "Base audit", "Base audit"},
		{"missing label filled", "", "Overlay audit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := Object{Name: "AuditArchive__c", SharingReasons: []string{"Authored_Audit__c"}}
			if test.baseLabel != "" {
				base.SharingReasonLabels = map[string]string{"Authored_Audit__c": test.baseLabel}
			}
			overlay := Object{Name: "AuditArchive__c", SharingReasons: []string{"authored_audit__c"}, SharingReasonLabels: map[string]string{"authored_audit__c": "Overlay audit"}}
			mergeObjectMetadata(&base, overlay)
			if !reflect.DeepEqual(base.SharingReasons, []string{"Authored_Audit__c"}) {
				t.Fatalf("merged token spelling changed: %#v", base.SharingReasons)
			}
			if got := base.SharingReasonLabel("Authored_Audit__c"); got != test.want {
				t.Fatalf("canonical token label = %q, want %q", got, test.want)
			}
			if got := base.SharingReasonLabel("authored_audit__c"); got != test.want {
				t.Fatalf("alternate-case token label = %q, want %q", got, test.want)
			}
			if len(base.SharingReasonLabels) != 1 || base.SharingReasonLabels["Authored_Audit__c"] != test.want {
				t.Fatalf("labels not keyed by retained token: %#v", base.SharingReasonLabels)
			}
			if got := base.SharingReasonLabel("Absent_Audit__c"); got != "" {
				t.Fatalf("missing token acquired label %q", got)
			}
		})
	}
}
