package vm

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestNetworkReflectionNativeVMAnonymousAndTestContextRoutes(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/network_configuration/reflection-org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	expressions := map[string]string{
		"qualifiedType":      "Type.forName('Schema.Network') != null",
		"unqualifiedType":    "Type.forName('Network') != null",
		"explicitSchemaType": "Type.forName('Schema','Network') != null",
		"memberAvailable":    "Schema.getGlobalDescribe().containsKey('NetworkMember')",
		"memberCount":        "Database.countQuery('SELECT COUNT() FROM NetworkMember')",
	}
	for _, row := range rows[1:] {
		t.Run(strings.Join(row[:4], "/"), func(t *testing.T) {
			program, err := CompileAnonymousWithOptions("System.assertEquals('"+row[4]+"',String.valueOf("+expressions[row[3]]+"));", CompileOptions{APIVersion: row[1]})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&org)
			if row[0] == "enabled" {
				storage.ApplyOrgShape(&org, []string{"Communities"})
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if row[2] == "istest" {
				machine.EnableTestContext()
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
