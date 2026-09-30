package resource

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestApplyProjectIndexesDocumentBodyAndReportMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/documents/OwnedDocs.documentFolder-meta.xml"), `<DocumentFolder><name>Owned Docs</name></DocumentFolder>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/documents/OwnedDocs/site.css"), "body { color: black; }\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/documents/OwnedDocs/site.css-meta.xml"), `<Document><name>site.css</name><public>true</public><internalUseOnly>false</internalUseOnly></Document>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/reports/OwnedReports.reportFolder-meta.xml"), `<ReportFolder><name>Owned Reports</name></ReportFolder>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/reports/OwnedReports/Upcoming.report-meta.xml"), `<Report><name>Upcoming reports</name><format>Tabular</format></Report>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	documents := org.Objects["Document"].Records
	if len(documents) != 1 {
		t.Fatalf("documents = %#v", documents)
	}
	for _, document := range documents {
		if got := document.Fields["Name"].String; got != "site.css" {
			t.Fatalf("Document.Name = %q", got)
		}
		if got := document.Fields["Body"]; got.Kind != storage.ValueBlob || got.String != "body { color: black; }\n" {
			t.Fatalf("Document.Body = %#v", got)
		}
		if !document.Fields["IsPublic"].Boolean || document.Fields["IsInternalUseOnly"].Boolean || document.Fields["FolderId"].Kind != storage.ValueID {
			t.Fatalf("Document flags = %#v", document.Fields)
		}
	}
	reports := org.Objects["Report"].Records
	if len(reports) != 1 {
		t.Fatalf("reports = %#v", reports)
	}
	for _, report := range reports {
		if got := report.Fields["DeveloperName"].String; got != "Upcoming" {
			t.Fatalf("Report.DeveloperName = %q", got)
		}
		if got := report.Fields["Name"].String; got != "Upcoming reports" {
			t.Fatalf("Report.Name = %q", got)
		}
		if got := report.Fields["Format"].String; got != "Tabular" {
			t.Fatalf("Report.Format = %q", got)
		}
	}
}

// Metadata API source uses folder metadata directly under documents/reports and
// XML .report files. The rows must retain their identities across project reloads.
func TestApplyProjectIndexesLegacyDocumentReportMetadataWithoutRebindingRows(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}]}`)
	writeFile(t, filepath.Join(root, "src/documents/Volunteers_Documents-meta.xml"), `<DocumentFolder><name>Volunteers Documents</name></DocumentFolder>`)
	writeFile(t, filepath.Join(root, "src/documents/Volunteers_Documents/VolunteersPersonalSiteCSS_css.css"), "body { color: black; }\n")
	writeFile(t, filepath.Join(root, "src/documents/Volunteers_Documents/VolunteersPersonalSiteCSS_css.css-meta.xml"), `<Document><name>VolunteersPersonalSiteCSS.css</name><public>true</public><internalUseOnly>false</internalUseOnly></Document>`)
	writeFile(t, filepath.Join(root, "src/reports/Volunteer_Reports-meta.xml"), `<ReportFolder><name>Volunteer Reports</name></ReportFolder>`)
	writeFile(t, filepath.Join(root, "src/reports/Volunteer_Reports/Upcoming_Volunteers.report"), `<Report><name>Upcoming Volunteers</name><format>Tabular</format></Report>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(p.FolderFiles); got != 2 {
		t.Fatalf("legacy folder files = %v", p.FolderFiles)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Document")
	storage.EnsureStandardObject(&org, "Report")
	docs := org.Objects["Document"]
	documentSeedID := storage.ID(docs.Definition.KeyPrefix + "000000000001AAA")
	docs.Records[documentSeedID] = storage.Record{ID: documentSeedID, Object: "Document", Fields: map[string]storage.Value{"Id": storage.IDValue(documentSeedID), "DeveloperName": storage.StringValue("Unrelated")}}
	org.Objects["Document"] = docs
	reports := org.Objects["Report"]
	reportSeedID := storage.ID(reports.Definition.KeyPrefix + "000000000001AAA")
	reports.Records[reportSeedID] = storage.Record{ID: reportSeedID, Object: "Report", Fields: map[string]storage.Value{"Id": storage.IDValue(reportSeedID), "DeveloperName": storage.StringValue("Unrelated")}}
	org.Objects["Report"] = reports
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	documentID := documentRecordID(t, org, "VolunteersPersonalSiteCSS")
	reportID := reportRecordID(t, org, "Volunteer_Reports", "Upcoming_Volunteers")
	document := org.Objects["Document"].Records[documentID]
	if got := document.Fields["Name"].String; got != "VolunteersPersonalSiteCSS.css" {
		t.Fatalf("legacy Document.Name = %q", got)
	}
	if got := document.Fields["Body"]; got.Kind != storage.ValueBlob || got.String != "body { color: black; }\n" {
		t.Fatalf("legacy Document.Body = %#v", got)
	}
	if !document.Fields["IsPublic"].Boolean || document.Fields["IsInternalUseOnly"].Boolean {
		t.Fatalf("legacy Document flags = %#v", document.Fields)
	}
	report := org.Objects["Report"].Records[reportID]
	if got := report.Fields["Name"].String; got != "Upcoming Volunteers" || report.Fields["Format"].String != "Tabular" {
		t.Fatalf("legacy Report = %#v", report.Fields)
	}
	if storage.IDsEqual(documentID, documentSeedID) || storage.IDsEqual(reportID, reportSeedID) {
		t.Fatalf("project rows did not skip equivalent 18-character IDs: document=%s report=%s", documentID, reportID)
	}
	if _, ok := org.Objects["Document"].Records[documentSeedID]; !ok {
		t.Fatal("seeded 18-character Document was replaced")
	}
	if _, ok := org.Objects["Report"].Records[reportSeedID]; !ok {
		t.Fatal("seeded 18-character Report was replaced")
	}

	writeFile(t, filepath.Join(root, "src/documents/Volunteers_Documents/Aardvark.css"), "a{}\n")
	writeFile(t, filepath.Join(root, "src/documents/Volunteers_Documents/Aardvark.css-meta.xml"), `<Document><name>Aardvark.css</name></Document>`)
	writeFile(t, filepath.Join(root, "src/reports/Volunteer_Reports/Aardvark.report"), `<Report><name>Aardvark</name><format>Summary</format></Report>`)
	p, err = project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if got := documentRecordID(t, org, "VolunteersPersonalSiteCSS"); got != documentID {
		t.Fatalf("Document identity rebound after earlier source: got %s want %s", got, documentID)
	}
	if got := reportRecordID(t, org, "Volunteer_Reports", "Upcoming_Volunteers"); got != reportID {
		t.Fatalf("Report identity rebound after earlier source: got %s want %s", got, reportID)
	}
	if got := len(org.Objects["Document"].Records); got != 3 {
		t.Fatalf("Document records = %d", got)
	}
	if got := len(org.Objects["Report"].Records); got != 3 {
		t.Fatalf("Report records = %d", got)
	}
}

func documentRecordID(t *testing.T, org storage.OrgState, developerName string) storage.ID {
	t.Helper()
	for id, record := range org.Objects["Document"].Records {
		if record.Fields["DeveloperName"].String == developerName {
			return id
		}
	}
	t.Fatalf("Document %q not found", developerName)
	return ""
}

func reportRecordID(t *testing.T, org storage.OrgState, folderName, developerName string) storage.ID {
	t.Helper()
	for id, record := range org.Objects["Report"].Records {
		if record.Fields["FolderName"].String == folderName && record.Fields["DeveloperName"].String == developerName {
			return id
		}
	}
	t.Fatalf("Report %q/%q not found", folderName, developerName)
	return ""
}
