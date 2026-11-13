package apextest

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/glade-sh/glade/internal/vm"
)

// NATIVE testdata/standard_plain_developer_token/org.tsv:2-3 anonymous 62.0 and 67.0.
func TestPlainDeveloperTokenCrossesContext(t *testing.T) {
    source, err := os.ReadFile(filepath.Join("testdata", "standard_plain_developer_token", "classes", "DescribeProbe.cls"))
    if err != nil { t.Fatal(err) }
    for _, api := range []string{"62.0", "67.0"} {
        t.Run(api, func(t *testing.T) {
            index := standardDescribeDeclaredBoundaryIndex(t, api, map[string]string{"DescribeProbe": string(source)}, nil)
            org := orgFromIndex(index)
            org.APIVersion = api
            machine := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org}).newMachine()
            program, err := vm.CompileAnonymousWithOptions(`Schema.DescribeFieldResult d=DescribeProbe.token().getDescribe(); String result=d.getName()+':'+d.getType()+':'+d.getLength()+':'+d.isAggregatable()+':'+d.getPrecision()+':'+d.getScale()+':'+d.getSoapType()+':'+d.isAutoNumber()+':'+d.isCascadeDelete()+':'+d.isCaseSensitive()+':'+d.isCustom()+':'+d.isDefaultedOnCreate()+':'+d.isDependentPicklist()+':'+d.isExternalId()+':'+d.isIdLookup()+':'+d.isNameField()+':'+d.isUnique();`, vm.CompileOptions{APIVersion: api})
            if err != nil { t.Fatal(err) }
            if _, err = machine.Execute(program); err != nil { t.Fatal(err) }
            want := "Tier:STRING:2:true:0:0:STRING:false:false:false:false:false:false:false:false:false:false"
            if got := machine.Globals["result"].Text; got != want { t.Fatalf("got %q want %q", got, want) }
        })
    }
}
