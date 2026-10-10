package resource

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestApplyProjectIndexesSourceFormatDocumentAndFollowingPage(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/documents/OwnedAssets.documentFolder-meta.xml"), `<DocumentFolder><name>Owned Assets</name></DocumentFolder>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/documents/OwnedAssets/OwnedPicture.document-meta.xml"), `<Document><name>OwnedPicture</name><public>true</public></Document>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/documents/OwnedAssets/OwnedPicture.png"), "owned image bytes")
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/OwnedLanding.page"), `<apex:page>Owned</apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/OwnedLanding.page-meta.xml"), `<ApexPage><apiVersion>62.0</apiVersion><label>Owned Landing</label></ApexPage>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if got := len(org.Objects["Document"].Records); got != 1 {
		t.Fatalf("document count = %d, want 1", got)
	}
	for _, row := range org.Objects["Document"].Records {
		if got := row.Fields["Body"]; got.Kind != storage.ValueBlob || got.String != "owned image bytes" {
			t.Fatalf("document body = %#v", got)
		}
		if got := row.Fields["BodyLength"]; got.Kind != storage.ValueInteger || got.Integer != int64(len("owned image bytes")) {
			t.Fatalf("document body length = %#v", got)
		}
		if row.Fields["DeveloperName"].String != "OwnedPicture" || row.Fields["Type"].String != "png" {
			t.Fatalf("document identity/type = %#v", row.Fields)
		}
	}
	if got := len(org.Objects["ApexPage"].Records); got != 1 {
		t.Fatalf("following page count = %d, want 1", got)
	}
}

func TestApplyProjectRejectsSourceDocumentWithoutUniqueBody(t *testing.T) {
	for _, tc := range []struct {
		name   string
		bodies []string
		want   string
	}{
		{name: "missing", want: "missing content file"},
		{name: "ambiguous", bodies: []string{"OwnedPicture.png", "OwnedPicture.gif"}, want: "multiple content files"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/documents/OwnedAssets.documentFolder-meta.xml"), `<DocumentFolder><name>Owned Assets</name></DocumentFolder>`)
			writeFile(t, filepath.Join(root, "force-app/main/default/documents/OwnedAssets/OwnedPicture.document-meta.xml"), `<Document><name>OwnedPicture</name></Document>`)
			for _, body := range tc.bodies {
				writeFile(t, filepath.Join(root, "force-app/main/default/documents/OwnedAssets", body), "owned body")
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
			if err := ApplyProject(&org, p); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ApplyProject error = %v, want %q", err, tc.want)
			}
		})
	}
}
