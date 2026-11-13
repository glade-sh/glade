package soql

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// NATIVE grouped spelling testdata/group_representative/org.tsv:2 anonymous 62.0
// NATIVE grouped spelling testdata/group_representative/org.tsv:3 istest 62.0
// NATIVE grouped spelling testdata/group_representative/org.tsv:4 anonymous 67.0
// NATIVE grouped spelling testdata/group_representative/org.tsv:5 istest 67.0
func TestGroupRepresentativeNative(t *testing.T) {
	f, err := os.Open("testdata/group_representative/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sets := [][]string{{"abc", "ABC"}, {"ABC", "abc"}, {"aBc", "ABC", "abc"}, {"abc", "aBc", "ABC"}, {"éclair", "ÉCLAIR", "eclair", "ECLAIR"}, {"ECLAIR", "eclair", "ÉCLAIR", "éclair"}, {"mix", "MIX", "Mix", "mIx"}}
	scan := bufio.NewScanner(f)
	scan.Scan()
	rows := 0
	for scan.Scan() {
		columns := strings.SplitN(scan.Text(), "\t", 4)
		if len(columns) != 4 {
			t.Fatal("invalid native row")
		}
		rows++
		t.Run(columns[0], func(t *testing.T) {
			expected := map[string]string{}
			for _, entry := range strings.Split(columns[3], "|") {
				kv := strings.SplitN(entry, "=", 2)
				if len(kv) != 2 {
					t.Fatal("invalid observation")
				}
				expected[kv[0]] = kv[1]
			}
			for i, values := range sets {
				check := func(values []string, key string, countedOnly bool) {
					org := collationOrg(values, "Account", map[string]bool{"Name": true})
					var parts []string
					queries := []string{"SELECT Name FROM Account GROUP BY Name", "SELECT Name, COUNT(Id) c FROM Account GROUP BY Name"}
					for n, q := range queries {
						if countedOnly && n == 0 {
							continue
						}
						result := collationQuery(t, org, "en_US", q)
						var groups []string
						for _, record := range result.Records {
							s := record.Fields["Name"].String
							if n == 1 {
								s += fmt.Sprintf(":%d", record.Fields["c"].Integer)
							}
							groups = append(groups, s)
						}
						sort.Strings(groups)
						parts = append(parts, strings.Join(groups, ","))
					}
					if got := strings.Join(parts, "/"); got != expected[key] {
						t.Errorf("%s got %q want %q", key, got, expected[key])
					}
				}
				check(values, fmt.Sprint(i), false)
				if i < 2 {
					swapped := []string{values[1], values[0]}
					check(swapped, fmt.Sprintf("swap%d", i), true)
				}
			}
		})
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if rows != 8 {
		t.Fatalf("got %d native rows, want 8", rows)
	}
}
