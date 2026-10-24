package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

type nativeBehaviorMatrixRow struct {
	Case, APIVersion, Route, Shape, Expected string
}

func TestNativeBehaviorMatrixConformance(t *testing.T) {
	const fixture = "testdata/conformance/native_behavior_matrix"
	raw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 27 {
		t.Fatalf("native matrix: rows=%d err=%v", len(rows), err)
	}

	native := map[string]nativeBehaviorMatrixRow{}
	for _, columns := range rows[1:] {
		if len(columns) != 5 || columns[0] == "" || columns[3] == "" || columns[4] == "TO_CAPTURE" {
			t.Fatalf("invalid native row: %v", columns)
		}
		row := nativeBehaviorMatrixRow{Case: columns[0], APIVersion: columns[1], Route: columns[2], Shape: columns[3], Expected: columns[4]}
		if row.APIVersion != "62.0" && row.APIVersion != "67.0" {
			t.Fatalf("unexpected API version in %s: %s", row.Case, row.APIVersion)
		}
		key := row.APIVersion + "/" + row.Route + "/" + row.Case
		if _, exists := native[key]; exists {
			t.Fatalf("duplicate native row: %s", key)
		}
		native[key] = row
	}

	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := copyNativeBehaviorProject(t, filepath.Join(fixture, "project"), api)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/MatrixNativeProbe.cls"), matrixNativeProbe(nativeBehaviorMatrixAssertion(native[api+"/istest/child_relationship_creation_order"])))
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/MatrixNativeProbe.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			index := loadTestIndex(t, root)
			for _, row := range native {
				if row.APIVersion != api || !nativeBehaviorMatrixAsserted(row.Case) {
					continue
				}
				t.Run(row.Route+"/"+row.Case, func(t *testing.T) {
					if row.Route == "istest" {
						method := "genericTypeIdentity"
						if row.Case == "child_relationship_creation_order" {
							method = "childRelationshipCreationOrder"
						}
						run := Run(index, Options{NoDiskCache: true, Parallelism: 1, SelectedClasses: []string{"MatrixNativeProbe"}, SelectedMethod: method})
						if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
							t.Fatalf("native test row: summary=%+v problem=%s", summary, firstRunProblem(run))
						}
						return
					}
					program, err := vm.CompileAnonymousWithOptions(nativeBehaviorMatrixAssertion(row), vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					if _, err := runner.execute(program); err != nil {
						t.Fatalf("native row %s actual behavior differs: %v", row.Case, err)
					}
				})
			}
		})
	}
}

func nativeBehaviorMatrixAsserted(caseName string) bool {
	switch caseName {
	case "null_list_iteration", "null_element_dereference", "child_relationship_order", "child_relationship_creation_order", "child_order_tie_query", "fieldset_member_paths", "feature_parameter_lookup", "generic_type_identity", "list_index_bounds", "formula_recalculation_cardinality":
		return true
	default:
		return false
	}
}

func nativeBehaviorMatrixAssertion(row nativeBehaviorMatrixRow) string {
	expected := conformanceApexString(row.Expected)
	switch row.Case {
	case "null_list_iteration":
		return "String observed; try { List<String> rows = null; for (String item : rows) { System.debug(item); } observed = 'completed'; } catch (Exception ex) { observed = ex.getTypeName() + '|' + ex.getMessage(); } System.assertEquals(" + expected + ", observed);"
	case "null_element_dereference":
		return "String observed; try { List<Account> rows = new List<Account>{null}; String value = rows[0].Name; observed = 'completed'; } catch (Exception ex) { observed = ex.getTypeName() + '|' + ex.getMessage(); } System.assertEquals(" + expected + ", observed);"
	case "child_relationship_order":
		var want []string
		if err := json.Unmarshal([]byte(row.Expected), &want); err != nil {
			panic(err)
		}
		return fmt.Sprintf("List<String> observed = new List<String>(); for (Schema.ChildRelationship relation : Orchard__c.SObjectType.getDescribe().getChildRelationships()) { String childName = relation.getChildSObject().getDescribe().getName(); if (childName == 'AlphaBranch__c' || childName == 'ZuluBranch__c') { observed.add(childName + '|' + relation.getField().getDescribe().getName() + '|' + relation.getRelationshipName()); } } System.assertEquals(%s, String.join(observed, ';'));", conformanceApexString(strings.Join(want, ";")))
	case "child_relationship_creation_order":
		var want []string
		if err := json.Unmarshal([]byte(row.Expected), &want); err != nil {
			panic(err)
		}
		return fmt.Sprintf("System.assertEquals(%s, String.join(CreationOrderProbe.observe(), ';'));", conformanceApexString(strings.Join(want, ";")))
	case "child_order_tie_query":
		var want []string
		if err := json.Unmarshal([]byte(row.Expected), &want); err != nil {
			panic(err)
		}
		return fmt.Sprintf("Orchard__c parent = new Orchard__c(Name = 'Matrix Parent'); insert parent; Date day = Date.newInstance(2024, 3, 7); insert new List<AlphaBranch__c>{new AlphaBranch__c(Name = 'First', AB_Orchard__c = parent.Id, VisitDate__c = day), new AlphaBranch__c(Name = 'Second', AB_Orchard__c = parent.Id, VisitDate__c = day), new AlphaBranch__c(Name = 'Third', AB_Orchard__c = parent.Id, VisitDate__c = day)}; List<String> observed = new List<String>(); for (AlphaBranch__c row : [SELECT Name FROM AlphaBranch__c WHERE AB_Orchard__c = :parent.Id ORDER BY VisitDate__c DESC]) { observed.add(row.Name); } System.assertEquals(%s, String.join(observed, ';'));", conformanceApexString(strings.Join(want, ";")))
	case "fieldset_member_paths":
		var want []string
		if err := json.Unmarshal([]byte(row.Expected), &want); err != nil {
			panic(err)
		}
		return fmt.Sprintf("List<String> observed = new List<String>(); for (Schema.FieldSetMember member : Orchard__c.SObjectType.getDescribe().fieldSets.getMap().get('Overview').getFields()) { observed.add(member.getFieldPath()); } System.assertEquals(%s, String.join(observed, ';'));", conformanceApexString(strings.Join(want, ";")))
	case "feature_parameter_lookup":
		return "String observed; try { Integer value = FeatureManagement.checkPackageIntegerValue('MissingMatrixFeature'); observed = 'value:' + String.valueOf(value); } catch (Exception ex) { observed = ex.getTypeName() + '|' + ex.getMessage(); } System.assertEquals(" + expected + ", observed);"
	case "list_index_bounds":
		return "String observed; try { List<String> rows = new List<String>(); String value = rows[0]; observed = 'completed'; } catch (Exception ex) { observed = ex.getTypeName() + '|' + ex.getMessage(); } System.assertEquals(" + expected + ", observed);"
	case "formula_recalculation_cardinality":
		return "Account account = new Account(Name = 'Matrix Formula Record'); insert account; List<Account> queried = [SELECT Id, Name FROM Account WHERE Id = :account.Id]; List<SObject> records = new List<SObject>(); records.addAll(queried); List<System.FormulaRecalcResult> observed = Formula.recalculateFormulas(records); System.assertEquals(" + expected + ", String.valueOf(observed.size()));"
	default:
		panic("unsupported anonymous native matrix row: " + row.Case)
	}
}

func matrixNativeProbe(creationOrderAssertion string) string {
	return `@IsTest private class MatrixNativeProbe {
    @IsTest static void genericTypeIdentity() {
        NativeBehaviorStubProvider provider = new NativeBehaviorStubProvider();
        NativeBehaviorTarget target = (NativeBehaviorTarget) Test.createStub(NativeBehaviorTarget.class, provider);
        target.accept(new List<Orchard__c>());
        System.assertEquals(true, provider.matchesLiteral);
        System.assertEquals('List<Orchard__c>', provider.parameterTypeName);
    }
    @IsTest static void childRelationshipCreationOrder() {
        ` + creationOrderAssertion + `
    }
}`
}

func copyNativeBehaviorProject(t *testing.T, sourceRoot, api string) string {
	t.Helper()
	root := t.TempDir()
	err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(filepath.Base(path), "NativeBehaviorProbe62.") || strings.HasPrefix(filepath.Base(path), "NativeBehaviorProbe67.") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		if strings.HasSuffix(path, ".cls-meta.xml") {
			text = "<ApexClass><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>"
		}
		if filepath.Base(path) == "sfdx-project.json" {
			text = fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api)
		}
		writeFile(t, filepath.Join(root, rel), text)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}
