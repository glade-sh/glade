package scripts

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestGoToolchainPins(t *testing.T) {
	const version = "1.26.10"
	for _, route := range []struct {
		path string
		want string
	}{
		{"go.mod", "\ngo " + version + "\n"},
		{"Dockerfile", "FROM golang:" + version + " AS build"},
		{"docs/AI_SETUP.md", "Go `" + version + "`"},
	} {
		t.Run(route.path, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", route.path))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), route.want) {
				t.Errorf("%s must pin %q", route.path, route.want)
			}
		})
	}
	for _, workflow := range []string{"ci", "race", "security", "release", "browser"} {
		t.Run(workflow+" workflow", func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", ".github", "workflows", workflow+".yml"))
			if err != nil {
				t.Fatal(err)
			}
			pins := regexp.MustCompile(`(?m)^\s+go-version: "([^"]+)"`).FindAllStringSubmatch(string(data), -1)
			if len(pins) == 0 {
				t.Fatal("workflow must pin its Go toolchain")
			}
			for _, pin := range pins {
				if pin[1] != version {
					t.Errorf("Go version = %s, want %s", pin[1], version)
				}
			}
		})
	}
}
