package gladecli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestRunDevLWCHelpUsesLWCHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(t.Context(), []string{"dev", "lwc", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"Start a local LWC development shell",
		"glade dev lwc [--project <root>] [--port <port>|--addr <host:port>] [--ready-file <path>]",
		"/lwc/preview/component/c/contextProbe",
		"/lwc/preview/tab/Lwc_Probe",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
}

func TestDevLWCRoutesListComponentsPagesTabsAndVisualforce(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
  <targets><target>lightning__RecordPage</target></targets>
</LightningComponentBundle>`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml"), `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <sobjectType>Account</sobjectType>
  <type>RecordPage</type>
</FlexiPage>`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/tabs/Lwc_Probe.tab-meta.xml"), `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>LWC Probe</label>
  <lwcComponent>c:contextProbe</lwcComponent>
</CustomTab>`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/tabs/VF_Probe.tab-meta.xml"), `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>VF Probe</label>
  <page>WidgetHost</page>
</CustomTab>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	routes := devLWCPreviewRoutes(p)
	got := strings.Join(routes, "\n")
	for _, want := range []string{
		"/lwc/preview/component/c/contextProbe",
		"/lwc/preview/record/Account/<recordId>?page=Account_Record_Page",
		"/lwc/preview/tab/Lwc_Probe",
		"/lwc/preview/tab/VF_Probe -> /apex/WidgetHost",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in routes:\n%s", want, got)
		}
	}
}

func TestWriteDevLWCReadyFileWritesShellRoutes(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
</LightningComponentBundle>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	readyPath := filepath.Join(t.TempDir(), "ready.json")
	if err := writeDevLWCReadyFile(readyPath, "127.0.0.1:39410", p); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(readyPath)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		URL    string   `json:"url"`
		Addr   string   `json:"addr"`
		Routes []string `json:"routes"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("readiness file is not JSON: %v\n%s", err, data)
	}
	if got.URL != "http://127.0.0.1:39410" || got.Addr != "127.0.0.1:39410" {
		t.Fatalf("ready = %#v", got)
	}
	if len(got.Routes) != 1 || got.Routes[0] != "/lwc/preview/component/c/contextProbe" {
		t.Fatalf("routes = %#v", got.Routes)
	}
}
