package dml

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestZZTitleProbe(t *testing.T) {
	dir := os.Getenv("PROBE_DIR")
	if dir == "" {
		t.Skip()
	}
	var cases map[string]map[string]string
	b, _ := os.ReadFile(dir + "/cases.json")
	json.Unmarshal(b, &cases)
	tsv, _ := os.ReadFile(dir + "/org.tsv")
	rowsObs := map[string]string{}
	for _, l := range strings.Split(strings.TrimSpace(string(tsv)), "\n")[1:] {
		f := strings.Split(l, "\t")
		if f[0] == "67.0" && f[1] == "anonymous" && f[2] == "1" {
			rowsObs[f[4]] = f[5]
		}
	}
	re := regexp.MustCompile(`'((?:[^'\\]|\\.)*)'`)
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	for _, g := range cases {
		for m, call := range g {
			if !strings.Contains(call, "Title") || !strings.HasPrefix(m, "tt") && !strings.HasPrefix(m, "t") {
				continue
			}
			if !strings.Contains(call, "FuzzyPanel") {
				continue
			}
			vals := re.FindAllStringSubmatch(call, -1)[3:]
			var titles []string
			for _, v := range vals {
				s := strings.ReplaceAll(v[1], `\'`, "'")
				titles = append(titles, s[strings.Index(s, "|")+1:])
			}
			nat := map[string]string{}
			_, body, _ := strings.Cut(rowsObs[m], ":")
			for _, tok := range strings.Split(body, ",") {
				if len(tok) >= 2 {
					c := "100"
					if len(tok) > 2 {
						f, _ := strconv.ParseFloat(tok[2:], 64)
						c = strconv.Itoa(int(f))
					}
					nat[tok[:2]] = c
				}
			}
			gl := map[string]string{}
			for i, a := range titles {
				for j, bb := range titles {
					if ok, c := fuzzyTitle(a, bb, false); ok {
						gl[string(digits[i])+string(digits[j])] = strconv.Itoa(int(c + 0.5))
					}
				}
			}
			keys := map[string]bool{}
			for k := range nat {
				keys[k] = true
			}
			for k := range gl {
				keys[k] = true
			}
			var ks []string
			for k := range keys {
				ks = append(ks, k)
			}
			sort.Strings(ks)
			n := 0
			for _, k := range ks {
				if nat[k] != gl[k] {
					i, j := strings.Index(digits, k[:1]), strings.Index(digits, k[1:])
					fmt.Printf("%s %s [%q -> %q] native=%s glade=%s\n", m, k, titles[i], titles[j], dash(nat[k]), dash(gl[k]))
					n++
				}
			}
			fmt.Printf("== %s: %d pairs, %d diffs\n", m, len(ks), n)
		}
	}
}
func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
