package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// The two walks orgShapeEvidenceScan replaced, kept as the reference.

func referenceSourceSettingsFiles(root string) []string {
	if root == "" {
		return nil
	}
	var found []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return nil
		}
		if path != root {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil || strings.Count(rel, string(os.PathSeparator)) >= personAccountEvidenceMaxDepth {
				return filepath.SkipDir
			}
		}
		if personAccountEvidenceSkipDirs[entry.Name()] || entry.Name() == "objects" {
			return filepath.SkipDir
		}
		if entry.Name() != "settings" {
			return nil
		}
		items, _ := os.ReadDir(path)
		for _, item := range items {
			if !item.IsDir() && strings.HasSuffix(item.Name(), sourceSettingsSuffix) {
				found = append(found, filepath.Join(path, item.Name()))
			}
		}
		return filepath.SkipDir
	})
	sort.Strings(found)
	return found
}

func referencePersonAccountRecordTypePaths(root string) []string {
	if root == "" {
		return nil
	}
	var found []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return nil
		}
		if path != root {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil || strings.Count(rel, string(os.PathSeparator)) >= personAccountEvidenceMaxDepth {
				return filepath.SkipDir
			}
		}
		name := entry.Name()
		if personAccountEvidenceSkipDirs[name] {
			return filepath.SkipDir
		}
		if name != "objects" {
			return nil
		}
		found = append(found, personAccountRecordTypesIn(path)...)
		return filepath.SkipDir
	})
	return found
}

func assertOrgShapeEvidenceScanMatches(t *testing.T, root string) {
	t.Helper()
	settings, personAccounts := orgShapeEvidenceScan(root)
	if want := referenceSourceSettingsFiles(root); !reflect.DeepEqual(settings, want) {
		t.Fatalf("%s: settings files = %v, want %v", root, settings, want)
	}
	if want := referencePersonAccountRecordTypePaths(root); !reflect.DeepEqual(personAccounts, want) {
		t.Fatalf("%s: person account paths = %v, want %v", root, personAccounts, want)
	}
}

func TestOrgShapeEvidenceScanMatchesSeparateWalks(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	personAccount := `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>PersonAccount</fullName></RecordType>`
	business := `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Business</fullName></RecordType>`
	write("force-app/main/default/settings/Account.settings-meta.xml", "<AccountSettings/>")
	write("force-app/main/default/settings/notes.txt", "")
	write("force-app/main/default/settings/nested/settings/Quote.settings-meta.xml", "<QuoteSettings/>")
	write("force-app/main/default/settings/objects/Account/recordTypes/PersonAccount.recordType-meta.xml", personAccount)
	write("force-app/main/default/objects/Account/recordTypes/PersonAccount.recordType-meta.xml", personAccount)
	write("force-app/main/default/objects/Account/recordTypes/Business.recordType-meta.xml", business)
	write("force-app/main/default/objects/PersonAccount/recordTypes/Any.recordType-meta.xml", "")
	write("force-app/main/default/objects/settings/Deep.settings-meta.xml", "<DeepSettings/>")
	write("force-app/main/default/classes/settings/Skipped.settings-meta.xml", "<SkippedSettings/>")
	write("unpackaged/settings/Case.settings-meta.xml", "<CaseSettings/>")
	write("unpackaged/objects/Account/recordTypes/PersonAccount.recordType-meta.xml", personAccount)
	write("node_modules/pkg/settings/Ignored.settings-meta.xml", "")
	deep := strings.Repeat("d/", personAccountEvidenceMaxDepth+1)
	write(deep+"settings/TooDeep.settings-meta.xml", "<TooDeepSettings/>")
	write(deep+"objects/Account/recordTypes/PersonAccount.recordType-meta.xml", personAccount)
	write("settings/Root.settings-meta.xml", "<RootSettings/>")

	assertOrgShapeEvidenceScanMatches(t, root)
	settings, personAccounts := orgShapeEvidenceScan(root)
	if len(settings) != 3 || len(personAccounts) != 4 {
		t.Fatalf("fixture coverage: settings %v, person accounts %v", settings, personAccounts)
	}
	for _, sub := range []string{"force-app/main/default/settings", "force-app/main/default/objects", "force-app/main/default/classes", "node_modules", "missing"} {
		assertOrgShapeEvidenceScanMatches(t, filepath.Join(root, filepath.FromSlash(sub)))
	}
	assertOrgShapeEvidenceScanMatches(t, "")
	if s, p := orgShapeEvidenceScan(""); s != nil || p != nil {
		t.Fatalf("empty root returned %v %v", s, p)
	}
	// The repository tree holds the fixture projects.
	assertOrgShapeEvidenceScanMatches(t, filepath.Join("..", ".."))
}
