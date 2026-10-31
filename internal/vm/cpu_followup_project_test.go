package vm_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestCPUFollowupProjectWarmMessage(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"sfdx-project.json": fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api),
				"force-app/main/default/classes/CpuProjectTest.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><status>Active</status></ApexClass>`,
				"force-app/main/default/classes/CpuProjectTest.cls":          `@IsTest private class CpuProjectTest { @IsTest static void exhaust() { Integer value = 1; } }`,
			}
			for name, body := range files {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			caps, _ := vm.LimitCapsForProfile("strict-sync")
			caps.CPUTimeMS = 0
			for run := 0; run < 2; run++ {
				report := apextest.Run(index, apextest.Options{SelectedClasses: []string{"CpuProjectTest"}, SelectedMethod: "exhaust", Parallelism: 1, LimitMode: vm.LimitModeStrict, LimitCaps: caps, LimitCapsSet: true})
				data, err := json.Marshal(report)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(data), "Apex CPU time limit exceeded") {
					t.Fatalf("run %d: %s", run, data)
				}
			}
		})
	}
}
