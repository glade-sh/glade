package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeRESTAPICLIContext(t *testing.T) {
	for _, mode := range []string{"--no-serve", "--daemon", "--connect"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newDaemonEquivalenceFixture(t, daemonEquivalenceScenario{classes: map[string]string{
				"RuntimeContext": `@IsTest private class RuntimeContext {
 private static String initial=JSON.serialize(new Account(Id='001000000000001'));
 @TestSetup static void setup(){insert new Account(Name='context setup',Description=initial);}
 private static void verify(String version){
  Account stored=[SELECT Description FROM Account WHERE Name='context setup' LIMIT 1];
  System.assert(stored.Description.contains('/services/data/v'+version+'/sobjects/Account/'),stored.Description);
  System.assert(initial.contains('/services/data/v'+version+'/sobjects/Account/'),initial);
 }
 @IsTest static void defaultContext(){verify('65.0');}
 @IsTest static void explicitContext(){verify('67.0');}
}`,
			}})
			writeDaemonEquivalenceFile(t, filepath.Join(fixture.projectRoot, "sfdx-project.json"), `{"sourceApiVersion":"53.0","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeDaemonEquivalenceFile(t, filepath.Join(fixture.projectRoot, "force-app/main/default/classes/RuntimeContext.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>`)
			if mode == "--connect" {
				stop := startDaemonEquivalenceServer(t, fixture.projectRoot)
				defer stop()
			}
			for _, version := range []string{"", "67.0", ""} {
				method := "defaultContext"
				effective := "65.0"
				args := []string{mode, "--class", "RuntimeContext", "--parallelism", "1"}
				if version != "" {
					method = "explicitContext"
					effective = version
					args = append(args, "--runtime-rest-api-version", version)
				}
				args = append(args, "--method", method)
				run := runNoCacheRoutingInvocation(t, fixture.projectRoot, args...)
				if run.RuntimeRESTAPIVersion != effective || run.Summary().Total != 1 || run.Summary().Passed != 1 {
					t.Fatalf("context lost: %#v", run)
				}
			}
			// Existing whole-class shard selection keeps the explicit runtime context.
			run := runNoCacheRoutingInvocation(t, fixture.projectRoot, mode, "--filter", "explicitContext", "--runtime-rest-api-version", "67.0", "--shard-count", "1", "--shard-index", "0", "--parallelism", "1")
			if run.RuntimeRESTAPIVersion != "67.0" || run.Summary().Total != 1 || run.Summary().Passed != 1 {
				t.Fatalf("shard context lost: %#v", run)
			}
		})
	}
	var output, errors bytes.Buffer
	exit := Run(context.Background(), []string{"test", "--project", t.TempDir(), "--runtime-rest-api-version", "999.0", "--json"}, &output, &errors)
	if exit == 0 || !strings.Contains(output.String()+errors.String(), "unsupported REST API version") {
		t.Fatalf("invalid context: exit=%d output=%s errors=%s", exit, &output, &errors)
	}
}

func TestRuntimeRESTAPIWatchOnce(t *testing.T) {
	fixture := newDaemonEquivalenceFixture(t, daemonEquivalenceScenario{classes: map[string]string{
		"RuntimeWatch": `@IsTest private class RuntimeWatch { @IsTest static void context(){String text=JSON.serialize(new Account(Id='001000000000001'));System.assert(text.contains('/services/data/v67.0/sobjects/Account/'),text);} }`,
	}})
	for _, mode := range []string{"--no-serve", "--daemon"} {
		var output, progress bytes.Buffer
		run, err := runTest(context.Background(), []string{"--project", fixture.projectRoot, mode, "--watch-once", "--runtime-rest-api-version", "67.0", "--parallelism", "1", "--no-progress"}, &output, &progress)
		if err != nil || run.Summary().Total != 1 || run.Summary().Passed != 1 || run.RuntimeRESTAPIVersion != "67.0" {
			t.Fatalf("watch %s context: %v %#v\n%s", mode, err, run, &output)
		}
	}
}
