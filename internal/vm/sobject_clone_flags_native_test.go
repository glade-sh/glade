package vm

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

func TestSObjectCloneFlagsNativeAnonymous(t *testing.T) {
	const fixture = "testdata/conformance/sobject_clone_flags"
	p, err := project.Load(filepath.Join(fixture, "project"))
	if err != nil {
		t.Fatal(err)
	}
	imported, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62", "67"} {
		data, err := os.ReadFile(filepath.Join(fixture, "native", "api"+api, "anonymous", "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(strings.NewReader(string(data)))
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows[1:] {
			name, expected := row[0], row[1]
			if !strings.HasPrefix(name, "single_") && !strings.HasPrefix(name, "list_") {
				continue
			}
			t.Run("api"+api+"/anonymous/"+name, func(t *testing.T) {
				parts := strings.Split(name, "_")
				args := ""
				if parts[1] != "default" {
					if parts[0] == "single" {
						args = strings.Join([]string{parts[1], parts[4], parts[2], parts[3]}, ",")
					} else {
						args = strings.Join(parts[1:], ",")
					}
				}
				expression := "source.clone(" + args + ")"
				if parts[0] == "list" {
					expression = "new List<CloneSample__c>{source}.deepClone(" + args + ")[0]"
				}
				source := `CloneSample__c source=(CloneSample__c)JSON.deserialize('{"Id":"a00000000000001AAA","Name":"Clone source","IsDeleted":false,"CreatedById":"005000000000001AAA","CreatedDate":"2001-02-03T04:05:06.000Z","LastModifiedById":"005000000000001AAA","LastModifiedDate":"2001-02-03T04:05:06.000Z","SystemModstamp":"2001-02-03T04:05:06.000Z","Sequence__c":"SEQ-0001"}',CloneSample__c.class);
CloneSample__c copied=` + expression + `;
List<String> names=new List<String>(copied.getPopulatedFieldsAsMap().keySet()); names.sort();
System.assertEquals('` + expected + `',String.join(names,','),'` + name + `');
System.assertNotEquals(null,source.CreatedDate);
System.assertEquals('SEQ-0001',source.Sequence__c);
System.assertEquals(source.SystemModstamp,copied.SystemModstamp);`
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api + ".0"})
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				for name, describe := range sobject.BuildDescribeRegistry(imported).Objects {
					org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(describe)}
				}
				machine := New(nil)
				machine.SetOrg(&org)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(fmt.Errorf("native %s: %w", name, err))
				}
			})
		}
	}
}

func TestSObjectCloneAuditDMLNativeRoutes(t *testing.T) {
	p, err := project.Load("testdata/conformance/sobject_clone_flags/project")
	if err != nil {
		t.Fatal(err)
	}
	imported, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []struct{ name, expression string }{
			{"singleDefault", "source.clone()"},
			{"singlePreserved", "source.clone(false,false,true,true)"},
			{"listDefault", "new List<CloneSample__c>{source}.deepClone()[0]"},
			{"json", "JSON.deserialize(JSON.serialize(source),CloneSample__c.class)"},
		} {
			for _, op := range []string{"insert", "update"} {
				for _, entry := range []string{"databasePartial", "databaseAllOrNone", "databaseList", "keyword"} {
					t.Run(api+"/anonymous/"+route.name+"/"+op+"/"+entry, func(t *testing.T) {
						org := storage.NewOrgState()
						for name, describe := range sobject.BuildDescribeRegistry(imported).Objects {
							org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(describe)}
						}
						source := `CloneSample__c seed=new CloneSample__c(Name='Audit source'); insert seed;
CloneSample__c source=[SELECT Name,CreatedDate,CreatedById,LastModifiedDate,LastModifiedById,SystemModstamp,IsDeleted,Sequence__c FROM CloneSample__c WHERE Id=:seed.Id];
CloneSample__c copied=(CloneSample__c)` + route.expression + `;`
						if op == "insert" {
							source += `copied.Id=null;`
						} else {
							source += `copied.Id=seed.Id;`
						}
						if entry == "keyword" {
							source += op + " copied;"
						} else {
							argument := "copied,false"
							suffix := ""
							if entry == "databaseAllOrNone" {
								argument = "copied,true"
							}
							if entry == "databaseList" {
								argument = "new List<CloneSample__c>{copied},false"
								suffix = "[0]"
							}
							source += `Database.SaveResult saved=Database.` + op + `(` + argument + `)` + suffix + `;
System.assertEquals(true,saved.isSuccess(),JSON.serialize(saved.getErrors()));`
						}
						program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						machine := New(nil)
						machine.SetOrg(&org)
						if _, err := machine.Execute(program); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}
