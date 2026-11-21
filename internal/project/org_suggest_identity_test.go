package project

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func adviceFixture(tb testing.TB, variant string, count int) string {
	tb.Helper()
	root := tb.TempDir()
	put := func(name, value string) {
		tb.Helper()
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(value), 0644); err != nil {
			tb.Fatal(err)
		}
	}
	put("sfdx-project.json", `{"packageDirectories":[{"path":"force-app"}]}`)
	source := `@IsTest class Sample%04d { @IsTest static void check() { List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = 'x']; System.assertEquals(1, 0); } %s }`
	extra := `static void features(Opportunity o) { o.CurrencyIsoCode='USD'; Account a = new Account(IsPersonAccount=true); String u = Site.getBaseUrl(); List<Network> n = [SELECT Id FROM Network]; Id x = Network.getNetworkId(); List<Territory2> t = [SELECT Id FROM Territory2]; }`
	for i := 0; i < count; i++ {
		features := ""
		if variant == "many" || variant == "benchmark" && i%20 == 0 {
			features = extra
		}
		put(fmt.Sprintf("force-app/main/default/classes/Sample%04d.cls", i), fmt.Sprintf(source, i, features))
	}
	if variant == "declared" {
		put("glade.yml", "org:\n  features: [PersonAccounts, MultiCurrency, Sites, Communities, NetworksEnabled]\n")
	}
	return root
}

func TestSuggestOrgSetupOutcomeIdentity(t *testing.T) {
	for _, variant := range []string{"none", "many", "declared"} {
		t.Run(variant, func(t *testing.T) {
			root := adviceFixture(t, variant, 4)
			got, err := json.MarshalIndent(SuggestOrgSetup(root), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')
			path := filepath.Join("testdata", "suggestion-outcomes", variant+".json")
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Fatalf("suggestions changed\ngot %s\nwant %s", got, want)
			}
		})
	}
}

func TestSuggestOrgSetupLargeOutcomeIdentity(t *testing.T) {
	root := adviceFixture(t, "benchmark", 658)
	got, err := json.Marshal(SuggestOrgSetup(root))
	if err != nil {
		t.Fatal(err)
	}
	if hash := fmt.Sprintf("%x", sha256.Sum256(got)); hash != "0f2cc8b435737d4009bd8f853ccbae769010003b605aee3b7753e5064bf405eb" {
		t.Fatalf("ordered suggestions changed: %s", got)
	}
}

func BenchmarkSuggestOrgSetupFailingSuite(b *testing.B) {
	root := adviceFixture(b, "benchmark", 658)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SuggestOrgSetup(root)
	}
}
