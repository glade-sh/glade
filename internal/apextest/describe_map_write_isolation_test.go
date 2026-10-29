package apextest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise private-holder and per-call lifetime controls through real project
// discovery, compilation, test invocation and worker schema caches at both APIs.
// Native read controls are in ../vm/testdata/describe_read_workload/org.tsv;
// local writes remain the explicit carry in that fixture's ORG-RUN.txt.
func TestDescribeMapWriteIsolationProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespace := range []string{"", "zzns"} {
			t.Run(api+"/"+namespace, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q,"namespace":%q}`, api, namespace))
				var methods strings.Builder
				id := 0
				for _, route := range []string{"getAppDescribe('app')", "getModuleDescribe('module')"} {
					for _, mutation := range []string{"remove('Account')", "clear()", "removeAll(new Set<String>{'Account'})", "retainAll(new Set<String>{'Contact'})"} {
						fmt.Fprintf(&methods, `@IsTest static void path%d() {
Map<String,Schema.SObjectType> holder = Schema.%s;
Map<String,Schema.SObjectType> sibling = Schema.%s;
Integer sizeBefore = sibling.size();
Set<String> keys = holder.keySet(); keys.%s;
System.assert(!holder.containsKey('Account'));
System.assertEquals(sizeBefore, sibling.size(), 'sibling changed through keySet');
System.assert(sibling.containsKey('Account'));
}`, id, route, route, mutation)
						id++
					}
				}
				for _, route := range []string{"getAppDescribe('app')", "getModuleDescribe('module')"} {
					for _, mutation := range []string{"remove('Account')", "clear()", "removeAll(new Set<String>{'Account'})", "retainAll(new Set<String>{'Contact'})"} {
						fmt.Fprintf(&methods, `@IsTest static void path%d() {
Schema.getGlobalDescribe(); Set<String> keys = Schema.%s.keySet(); keys.%s;
System.assert(!keys.contains('Account')); System.assert(Schema.%s.containsKey('Account'));
}`, id, route, mutation, route)
						id++
					}
				}

				if namespace != "" {
					for _, route := range []string{"getAppDescribe('app')", "getModuleDescribe('module')"} {
						fmt.Fprintf(&methods, `@IsTest static void path%d() {
Schema.getGlobalDescribe(); Schema.%s.put('private',Account.SObjectType);
System.assert(Schema.getGlobalDescribe().containsKey('private'));
}`, id, route)
						id++
					}

					for _, mutation := range []string{"put('private',Account.SObjectType)", "putAll(new Map<String,Schema.SObjectType>{'private'=>Account.SObjectType})", "remove('account')", "clear()"} {
						fmt.Fprintf(&methods, `@IsTest static void path%d() {
Integer sizeBefore = Schema.getGlobalDescribe().size();
Schema.getGlobalDescribe().%s;
System.assert(!Schema.getGlobalDescribe().containsKey('private'));
System.assertEquals(sizeBefore, Schema.getGlobalDescribe().size());
System.assertEquals(Account.SObjectType, Schema.getGlobalDescribe().get('account'));
}`, id, mutation)
						id++
					}
				}
				classes := filepath.Join(root, "force-app/main/default/classes")
				writeFile(t, filepath.Join(classes, "DescribeMapIsolationTest.cls"), "@IsTest private class DescribeMapIsolationTest {"+methods.String()+"}")
				writeFile(t, filepath.Join(classes, "DescribeMapIsolationTest.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				index := loadTestIndex(t, root)
				for _, parallelMethods := range []bool{false, true} {
					run := Run(index, Options{Parallelism: 1, ParallelMethods: parallelMethods, NoDiskCache: true})
					if summary := run.Summary(); summary.Total != id || summary.Passed != id {
						t.Fatalf("parallelMethods=%v summary=%+v suites=%+v", parallelMethods, summary, run.Suites)
					}
				}
			})
		}
	}
}
