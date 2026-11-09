package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// eagerExecProjectLoader builds the exec org the way the full-catalog start
// did: every standard object installed before the first statement runs.
func eagerExecProjectLoader(root string, loadOrg bool) execProjectLoad {
	load := loadExecProject(root, loadOrg)
	if load.err == nil && load.orgErr == nil && loadOrg {
		storage.MaterializeStandardObjectSpread(&load.org)
	}
	return load
}

func execJSONData(t *testing.T, root, source string, loader func(string, bool) execProjectLoad) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := runExecWithProjectLoader(context.Background(), []string{"--project", root, "--json", source}, &out, loader)
	if err != nil {
		return "", err
	}
	var envelope struct {
		Status string `json:"status"`
		Data   struct {
			Debug []string       `json:"debug"`
			Vars  map[string]any `json:"vars"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("exec JSON: %v\n%s", err, out.String())
	}
	normalized, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return string(normalized), nil
}

// The exec org adds catalog objects on first reference. Every answer that
// depends on the schema must match an org that installed the whole catalog up
// front: counts, describes, global describe, child relationships, type and Id
// lookups, and the feature gates.
func TestRunExecLazySpreadMatchesEagerSpread(t *testing.T) {
	for _, shape := range []struct{ name, config string }{
		{"no inputs", ""},
		{"communities", "org:\n  features: [Communities]\n"},
		{"communities quotes multicurrency", "org:\n  features: [Communities, NetworksEnabled, Quotes, MultiCurrency]\n"},
	} {
		t.Run(shape.name, func(t *testing.T) {
			root := execStandardOrgProject(t, shape.config)
			for _, source := range []string{
				`System.debug([SELECT COUNT() FROM AsyncApexJob]); System.debug([SELECT COUNT() FROM CronTrigger]); System.debug([SELECT COUNT() FROM OrgWideEmailAddress]);`,
				`System.debug(Schema.getGlobalDescribe().size()); System.debug(Schema.getGlobalDescribe().containsKey('quote')); System.debug(Schema.getGlobalDescribe().containsKey('network'));`,
				`System.debug(Schema.SObjectType.Account.getChildRelationships().size()); System.debug(Schema.SObjectType.Opportunity.getChildRelationships().size());`,
				`System.debug(OpportunityLineItem.SObjectType.getDescribe().fields.getMap().size()); System.debug(PricebookEntry.SObjectType.getDescribe().getKeyPrefix()); System.debug(EmailTemplate.SObjectType.getDescribe().getLabelPlural());`,
				`System.debug([SELECT Id, (SELECT Id FROM OpportunityLineItems) FROM Opportunity].size()); System.debug([SELECT Id, (SELECT Id FROM GroupMembers) FROM Group].size());`,
				`Id job = '707000000000001AAA'; System.debug(job.getSObjectType()); System.debug(Type.forName('AsyncApexJob')); System.debug(Type.forName('ApexClass'));`,
				`Group g = new Group(Name='Lazy', Type='Regular'); insert g; insert new GroupMember(GroupId=g.Id, UserOrGroupId=UserInfo.getUserId()); System.debug([SELECT COUNT() FROM GroupMember WHERE GroupId = :g.Id]); System.debug(String.valueOf(g.Id).substring(0, 3));`,
				`try { System.debug([SELECT COUNT() FROM Quote]); } catch (QueryException e) { System.debug(e.getMessage()); } try { System.debug([SELECT COUNT() FROM Network]); } catch (QueryException e) { System.debug(e.getMessage()); }`,
				`try { System.debug([SELECT COUNT() FROM CurrencyType]); } catch (QueryException e) { System.debug(e.getMessage()); } System.debug(Schema.SObjectType.Opportunity.fields.getMap().containsKey('currencyisocode'));`,
				`System.debug(Schema.describeSObjects(new List<String>{'ApexClass', 'EmailTemplate', 'Pricebook2'}).size()); System.debug(new AsyncApexJob().getSObjectType()); System.debug(Schema.SObjectType.Task.fields.WhatId.getDescribe().getReferenceTo().size());`,
				// Semi-join and anti-join subqueries on cold objects, static and dynamic.
				`System.debug([SELECT COUNT() FROM Account WHERE Id IN (SELECT AccountId FROM Entitlement)]); System.debug([SELECT COUNT() FROM Account WHERE Id NOT IN (SELECT AccountId FROM Entitlement)]); System.debug([SELECT COUNT() FROM Contact WHERE Id IN (SELECT ContactId FROM ServiceContract)]);`,
				`System.debug(Database.query('SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Entitlement WHERE Id IN (SELECT EntitlementId FROM ServiceContract))').size()); System.debug(Database.countQuery('SELECT COUNT() FROM Contact WHERE Id NOT IN (SELECT ContactId FROM ServiceContract)'));`,
				// TYPEOF on cold types.
				`System.debug([SELECT TYPEOF What WHEN Account THEN Name WHEN Solution THEN SolutionName END FROM Task].size()); System.debug([SELECT Id, TYPEOF Owner WHEN User THEN Username WHEN Group THEN Name END FROM Case].size());`,
				// Captured-only key prefixes: Entitlement 550, ServiceContract 810, Site 0DM, ServiceResource 0Hn.
				`Id e = '550000000000001AAA'; System.debug(e.getSObjectType()); Id s = Id.valueOf('810000000000001AAA'); System.debug(s.getSObjectType()); System.debug(((Id) '0DM000000000001AAA').getSObjectType()); Id r = '0Hn000000000001AAA'; System.debug(r.getSObjectType().newSObject(r));`,
				`Id s = '810000000000001AAA'; Database.DeleteResult one = Database.delete(s, false); System.debug(one.isSuccess() + ' ' + one.getErrors()[0].getStatusCode()); List<Database.DeleteResult> many = Database.delete(new List<Id>{'550000000000001AAA', '0Hn000000000001AAA'}, false); System.debug(many.size() + ' ' + many[1].getErrors()[0].getStatusCode());`,
			} {
				eager, eagerErr := execJSONData(t, root, source, eagerExecProjectLoader)
				lazy, lazyErr := execJSONData(t, root, source, loadExecProject)
				if (eagerErr == nil) != (lazyErr == nil) || (eagerErr != nil && eagerErr.Error() != lazyErr.Error()) {
					t.Errorf("%s\n eager err %v\n lazy err  %v", source, eagerErr, lazyErr)
					continue
				}
				if eager != lazy {
					t.Errorf("%s\n eager %s\n lazy  %s", source, eager, lazy)
				}
			}
		})
	}
}

// A minimal project must not pay for the catalog at startup: the org starts
// lean and marks the spread as pending, so exec stays close to its cost before
// the spread was shared with glade test.
func TestRunExecProjectOrgStartsLean(t *testing.T) {
	root := execStandardOrgProject(t, "")
	load := loadExecProject(root, true)
	if load.err != nil || load.orgErr != nil {
		t.Fatalf("load: %v %v", load.err, load.orgErr)
	}
	if !load.org.LazyStandardSpreadEnabled() {
		t.Fatal("exec org does not mark the standard spread as lazy")
	}
	if n := len(load.org.Objects); n > 400 {
		t.Fatalf("exec org starts with %d objects; the catalog must stay lazy", n)
	}
	eager := load.org.Clone()
	if !storage.MaterializeStandardObjectSpread(&eager) {
		t.Fatal("materialize reported no change")
	}
	if len(eager.Objects) < 10*len(load.org.Objects) {
		t.Fatalf("materialized spread has %d objects, lean org %d", len(eager.Objects), len(load.org.Objects))
	}
}

// Startup cost of the exec org on a minimal project. Compare against a build
// that installs the whole catalog with:
//
//	go test ./internal/gladecli -run '^$' -bench 'ExecProjectOrgStartup' -benchmem
func BenchmarkExecProjectOrgStartup(b *testing.B) {
	root := b.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`), 0o644); err != nil {
		b.Fatal(err)
	}
	b.Run("lazy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			load := loadExecProject(root, true)
			if load.err != nil || load.orgErr != nil {
				b.Fatal(load.err, load.orgErr)
			}
		}
	})
	b.Run("exec", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := runExec(context.Background(), []string{"--project", root, "--json", "Integer n = [SELECT COUNT() FROM Account];"}, io.Discard); err != nil {
				b.Fatal(err)
			}
		}
	})
}
