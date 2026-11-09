package apextest

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// capi3_knowledge_community/org.tsv: a 0DB communityId reaches the data-siloed
// boundary whether or not a Network of that id exists, in every org shape, while
// 'me', other prefixes, a User id and a malformed 0DB id are rejected.
func TestCapi3KnowledgeCommunityIDs(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "conformance", "capi3_knowledge_community", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	type key struct{ api, org string }
	byOrg := map[key]map[string]string{}
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 4 || p[0] == "api" || !strings.HasPrefix(p[2], "top.") {
			continue
		}
		k := key{p[0], p[1]}
		if byOrg[k] == nil {
			byOrg[k] = map[string]string{}
		}
		byOrg[k][strings.TrimPrefix(p[2], "top.")] = p[3]
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	literals := map[string]map[string]string{}
	lf, err := os.Open(filepath.Join("testdata", "conformance", "capi3_knowledge_community", "internal.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer lf.Close()
	lscan := bufio.NewScanner(lf)
	for lscan.Scan() {
		p := strings.Split(lscan.Text(), "\t")
		if len(p) != 5 || p[2] != "top" {
			continue
		}
		if literals[p[0]] == nil {
			literals[p[0]] = map[string]string{}
		}
		literals[p[0]][strings.TrimSuffix(strings.TrimPrefix(p[3], "["), "]")] = p[4]
	}
	if err := lscan.Err(); err != nil {
		t.Fatal(err)
	}
	inputs := map[string]string{
		"null": "null", "fake0DB18": "'0DB000000000001AAA'", "fake0DB15": "'0DB000000000001'",
		"fake0DBother": "'0DB0X0000000099AAA'", "internal": "'internal'", "me": "'me'",
		"fake001": "'001000000000001AAA'", "userId": "String.valueOf(UserInfo.getUserId())",
		"fake0DBmalformed": "'0DB00000000000'", "realId18": "realId", "realId15": "realId.left(15)",
	}
	const siloed = "System.UnsupportedOperationException|ConnectApi methods are not supported in data siloed tests. Please use @IsTest(SeeAllData=true)."
	shapes := []struct {
		label, name, scratchDef string
		network                 bool
	}{
		{"Developer scratch: knowledgeSettings.enableKnowledge only", "knowledge-only", `{"features":["Knowledge"],"settings":{"knowledgeSettings":{"enableKnowledge":true}}}`, false},
		{"Developer scratch: knowledge + communitiesSettings.enableNetworksEnabled, no site", "networks-no-site", `{"features":["Knowledge","Communities"],"settings":{"knowledgeSettings":{"enableKnowledge":true},"communitiesSettings":{"enableNetworksEnabled":true}}}`, false},
		{"Developer scratch: knowledge + networks setting + one site (Synthetic Portal)", "networks-site", `{"features":["Knowledge","Communities"],"settings":{"knowledgeSettings":{"enableKnowledge":true},"communitiesSettings":{"enableNetworksEnabled":true}}}`, true},
		{"Enterprise trial org (no Network object)", "unspecified", "", false},
	}
	network, err := os.ReadFile(filepath.Join("testdata", "conformance", "network_configuration", "Synthetic Portal.network-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, shape := range shapes {
			rows := byOrg[key{api, shape.label}]
			if len(rows) < 9 {
				t.Fatalf("%s %s: native rows=%d", api, shape.name, len(rows))
			}
			t.Run(api+"/"+shape.name, func(t *testing.T) {
				p := newSalesforceSurfaceProject(t, `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				if shape.scratchDef != "" {
					writeFile(t, filepath.Join(p.root, "config/project-scratch-def.json"), shape.scratchDef)
				}
				if shape.network {
					writeFile(t, filepath.Join(p.root, "force-app/main/default/networks/Synthetic Portal.network-meta.xml"), string(network))
				}
				var b strings.Builder
				b.WriteString(`@IsTest private class CommunityIdTest {
  static String top(String c) { try { ConnectApi.Knowledge.getTopViewedArticlesForTopic(c, '0TO000000000001', 1); return 'OK'; } catch (Exception e) { return e.getTypeName() + '|' + e.getMessage().replace(UserInfo.getUserId(), '{userId}'); } }
  @IsTest static void probe() {
    String realId = null;
    if (Schema.getGlobalDescribe().containsKey('Network')) { List<SObject> nets = Database.query('SELECT Id FROM Network ORDER BY Name'); if (!nets.isEmpty()) realId = String.valueOf(nets[0].get('Id')); }
`)
				for name, expr := range inputs {
					want, ok := rows[name]
					if !ok {
						continue
					}
					if want == "SILOED" {
						want = siloed
					}
					fmt.Fprintf(&b, "    System.assertEquals('%s', top(%s), '%s');\n", strings.ReplaceAll(want, "'", `\'`), expr, name)
				}
				if len(literals[api]) != 7 {
					t.Fatalf("%s: literal rows=%d", api, len(literals[api]))
				}
				for value, want := range literals[api] {
					if strings.HasPrefix(want, "System.UnsupportedOperationException") {
						want = siloed
					}
					fmt.Fprintf(&b, "    System.assertEquals('%s', top('%s'), 'literal [%s]');\n", strings.ReplaceAll(want, "'", `\'`), value, value)
				}
				b.WriteString("  }\n}")
				p.writeClass("CommunityIdTest", b.String())
				run := p.run()
				if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
					t.Fatalf("summary=%#v problem=%q", got, firstRunProblem(run))
				}
			})
		}
	}
}
