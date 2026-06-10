package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/typesys"
)

func main() {
	root := "/Users/matt/Radical1/Projects/src-nmb-nu"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	cpuPath := filepath.Join(os.TempDir(), "glade-profile-warm-cpu.pprof")
	if v := os.Getenv("GLADE_CPU_PROFILE"); v != "" {
		cpuPath = v
	}
	cpu, err := os.Create(cpuPath)
	if err != nil {
		panic(err)
	}
	defer cpu.Close()
	if err := pprof.StartCPUProfile(cpu); err != nil {
		panic(err)
	}
	defer pprof.StopCPUProfile()

	type phase struct {
		Name string `json:"name"`
		MS   int64  `json:"ms"`
	}
	var phases []phase
	record := func(name string, start time.Time) {
		phases = append(phases, phase{Name: name, MS: time.Since(start).Milliseconds()})
	}

	t0 := time.Now()
	p, err := project.Load(root)
	if err != nil {
		panic(err)
	}
	record("project.Load", t0)

	t1 := time.Now()
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		panic(err)
	}
	record("schema.LoadProject", t1)

	t2 := time.Now()
	index := typesys.Build(p, s)
	record("typesys.Build", t2)

	cachePath := filepath.Join(root, ".glade", "test", "startup.gob")
	var cacheSize int64
	if st, err := os.Stat(cachePath); err == nil {
		cacheSize = st.Size()
	}

	t3 := time.Now()
	entry, err := startupcache.Read(root, startupcache.SubdirTest)
	record("startupcache.Read(gob)", t3)
	if err != nil {
		panic(err)
	}
	if entry != nil {
		_ = entry
	}

	t6 := time.Now()
	apextest.InvalidateRuntimeCaches()
	apextest.WarmRuntime(index)
	record("apextest.WarmRuntime(cold)", t6)

	if cacheSize > 0 {
		apextest.InvalidateRuntimeCaches()
		t7 := time.Now()
		apextest.WarmRuntime(index)
		record("apextest.WarmRuntime(disk-cache)", t7)
	}

	out := map[string]any{
		"project":     root,
		"cachePath": cachePath,
		"cacheMB":   cacheSize / (1 << 20),
		"types":     len(index.Types),
		"phases":    phases,
		"cpuProfile": cpuPath,
	}
	blob, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(blob))
}
