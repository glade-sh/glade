//go:build ignore

// Generate the root table from the pinned CLDR weights.
package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/collate/build"
)

func main() {
	raw, err := os.ReadFile("collationdata/cldr_root.txt")
	if err != nil {
		panic(err)
	}
	builder := build.NewBuilder()
	elements := regexp.MustCompile(`\[([.*])([0-9A-F]+)\.([0-9A-F]+)\.([0-9A-F]+)\]`)
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' || line[0] == '@' {
			continue
		}
		text, weights, ok := strings.Cut(line, ";")
		if !ok {
			panic("invalid root weight row")
		}
		var runes []rune
		for _, token := range strings.Fields(text) {
			n, err := strconv.ParseInt(token, 16, 32)
			if err != nil {
				panic(err)
			}
			runes = append(runes, rune(n))
		}
		var ces [][]int
		var variables []int
		for i, match := range elements.FindAllStringSubmatch(weights, -1) {
			ce := make([]int, 3)
			for j := range ce {
				n, err := strconv.ParseInt(match[j+2], 16, 32)
				if err != nil {
					panic(err)
				}
				ce[j] = int(n)
			}
			// UCA's single FFFD primary is beyond the builder's ordinary-primary
			// encoding. Its public Add contract allows weight renumbering: encode
			// this last regular primary with the builder's highest-weight escape.
			// FFFE itself has primary 1 in CLDR and remains at the beginning.
			if ce[0] == 0xfffd {
				ce[0] = 0xffff
			}
			ces = append(ces, ce)
			if match[1] == "*" {
				variables = append(variables, i)
			}
		}
		if err := builder.Add(runes, ces, variables); err != nil {
			panic(fmt.Sprintf("root table row %s: %v", text, err))
		}
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	table, err := builder.Build()
	if err != nil {
		panic(err)
	}
	if len(os.Args) > 2 {
		reference, err := json.Marshal(table)
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(os.Args[2], reference, 0644); err != nil {
			panic(err)
		}
	}
	var output bytes.Buffer
	if err := gob.NewEncoder(&output).Encode(table); err != nil {
		panic(err)
	}
	encoded := base64.StdEncoding.EncodeToString(output.Bytes()) + "\n"
	outputPath := "collationdata/cldr_root_table.b64"
	if len(os.Args) > 1 {
		outputPath = os.Args[1]
	}
	if err := os.WriteFile(outputPath, []byte(encoded), 0644); err != nil {
		panic(err)
	}
}
