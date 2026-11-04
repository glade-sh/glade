package gladecli

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestDevSelectorHelpDescribesFullTestName(t *testing.T) {
	for _, route := range []string{"public", "renderer"} {
		t.Run(route, func(t *testing.T) {
			var out, stderr bytes.Buffer
			if route == "public" {
				if code := Run(context.Background(), []string{"help", "dev"}, &out, &stderr); code != 0 {
					t.Fatalf("exit %d: %s", code, stderr.String())
				}
			} else if _, _, err := runDev(context.Background(), []string{"--help"}, &out, &stderr); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"full Class.method test name", "case-insensitive", "comma-separated alternatives", "watch", "--class refund"} {
				if !strings.Contains(out.String(), want) {
					t.Errorf("%s help missing %q: %s", route, want, out.String())
				}
			}
			if route == "public" {
				for _, flag := range []string{"--class", "--test"} {
					found := false
					for _, line := range strings.Split(out.String(), "\n") {
						if strings.HasPrefix(strings.TrimSpace(line), flag+" ") && strings.Contains(line, "full Class.method test name") {
							found = true
						}
					}
					if !found {
						t.Errorf("%s help does not describe full-name matching", flag)
					}
				}
			}
		})
	}
}

func TestDevSelectorFiltersFullTestNames(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeTestFile(t, filepath.Join(root, "force-app/main/classes/OrdersTest.cls"), `@isTest private class OrdersTest { @isTest static void checksRefund() { System.assert(true); } @isTest static void checksOrder() { System.assert(true); } }`)
			writeTestFile(t, filepath.Join(root, "force-app/main/classes/BillingTest.cls"), `@isTest private class BillingTest { @isTest static void handlesRefund() { System.assert(true); } @isTest static void handlesPayment() { System.assert(true); } }`)
			for _, tc := range []struct {
				flag, filter string
				want         []string
			}{
				{"--class", "REFUND", []string{"BillingTest.handlesRefund", "OrdersTest.checksRefund"}},
				{"--test", "orders", []string{"OrdersTest.checksOrder", "OrdersTest.checksRefund"}},
				{"--test", "checksorder, handlespayment", []string{"BillingTest.handlesPayment", "OrdersTest.checksOrder"}},
			} {
				t.Run(tc.flag+"/"+tc.filter, func(t *testing.T) {
					var out bytes.Buffer
					run, err := runDevTest(context.Background(), []string{"--project", root, "--out", filepath.Join(t.TempDir(), "runs"), tc.flag, tc.filter, "--no-progress"}, &out, io.Discard)
					if err != nil {
						t.Fatal(err)
					}
					var got []string
					for _, suite := range run.Suites {
						for _, c := range suite.Cases {
							if string(c.Status) != "pass" {
								t.Fatalf("%s.%s status %s", c.ClassName, c.MethodName, c.Status)
							}
							got = append(got, c.ClassName+"."+c.MethodName)
						}
					}
					sort.Strings(got)
					if !reflect.DeepEqual(got, tc.want) {
						t.Fatalf("selected %v, want %v", got, tc.want)
					}
				})
			}
		})
	}
}
