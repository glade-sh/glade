package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// BenchmarkLightningWirePair measures the two independent consumers over real
// HTTP connections, including serialization at the server request boundary.
func BenchmarkLightningWirePair(b *testing.B) {
	handler := lightningWireBenchmarkServer(b)
	orgBytes, err := json.Marshal(handler.Org)
	if err != nil {
		b.Fatal(err)
	}
	fields, records := 0, 0
	for _, object := range handler.Org.Objects {
		fields += len(object.Definition.Fields)
		records += len(object.Records)
	}
	local := httptest.NewServer(handler)
	defer local.Close()
	body := []byte(`{"className":"FamilyL07Oracle","method":"echo","params":{"marker":"owned","ordinal":1},"cacheable":true}`)
	request := func() (time.Duration, error) {
		start := time.Now()
		response, err := local.Client().Post(local.URL+"/lightning/wire/apex", "application/json", bytes.NewReader(body))
		if err != nil {
			return 0, err
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		if err != nil {
			return 0, err
		}
		var out lwcbrowser.WireResponse
		if err = json.Unmarshal(data, &out); err != nil {
			return 0, err
		}
		if response.StatusCode != http.StatusOK || out.Error != nil || out.Data == nil {
			b.Errorf("wire response: %s", data)
		}
		return time.Since(start), nil
	}
	// Warm declaration caches and both connections outside the measurement.
	warm := make(chan error, 2)
	for range 2 {
		go func() { _, err := request(); warm <- err }()
	}
	for range 2 {
		if err := <-warm; err != nil {
			b.Fatal(err)
		}
	}
	durations := make([]time.Duration, 0, 2*b.N)
	secondFirst, propertyFirst := 0, 0
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		type result struct {
			issued  int32
			index   int
			elapsed time.Duration
			err     error
		}
		done := make(chan result, 2)
		start := make(chan struct{})
		var ready sync.WaitGroup
		ready.Add(2)
		var issued atomic.Int32
		for index := 0; index < 2; index++ {
			go func() {
				ready.Done()
				<-start
				sequence := issued.Add(1)
				elapsed, err := request()
				done <- result{issued: sequence, index: index, elapsed: elapsed, err: err}
			}()
		}
		ready.Wait()
		close(start)
		first, second := <-done, <-done
		if first.err != nil || second.err != nil {
			b.Fatalf("wire requests: %v %v", first.err, second.err)
		}
		if first.issued == 2 {
			secondFirst++
		}
		if first.index == 0 {
			propertyFirst++
		}
		durations = append(durations, first.elapsed, second.elapsed)
	}
	b.StopTimer()
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	b.ReportMetric(float64(durations[len(durations)/2].Microseconds()), "median-us/request")
	b.ReportMetric(float64(secondFirst)*100/float64(b.N), "issue-reversed-%")
	b.ReportMetric(float64(propertyFirst)*100/float64(b.N), "property-first-%")
	b.ReportMetric(float64(2*b.N), "requests")
	b.ReportMetric(float64(len(orgBytes)), "org-bytes")
	b.ReportMetric(float64(fields), "org-fields")
	b.ReportMetric(float64(records), "org-records")
	b.ReportMetric(float64(len(handler.runtime.Classes)), "runtime-classes")
}

func lightningWireBenchmarkServer(b *testing.B) *Server {
	b.Helper()
	p, err := project.Load(filepath.Join("..", "lwc", "compile", "testdata", "l07_runtime", "api67"))
	if err != nil {
		b.Fatal(err)
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		b.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		b.Fatal(err)
	}
	registry, err := resource.LoadProject(p)
	if err != nil {
		b.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	org.Metadata = registry
	handler := NewWithSource(&org, source)
	handler.SetProjectIndex(typesys.Build(p, schema))
	return handler
}

// BenchmarkLightningApexExecution excludes HTTP, org cloning, and request
// runtime construction to isolate the method invocation and return coercion.
func BenchmarkLightningApexExecution(b *testing.B) {
	handler := lightningWireBenchmarkServer(b)
	machine, err := handler.visualforceRuntime()
	if err != nil {
		b.Fatal(err)
	}
	params := map[string]any{"marker": "owned", "ordinal": 1}
	b.ResetTimer()
	for range b.N {
		result, err := machine.InvokeLWCMethod("FamilyL07Oracle", "echo", params)
		if err != nil || !result.Success {
			b.Fatalf("invoke: %v %+v", err, result)
		}
	}
}

// BenchmarkLightningWireRequestSetup separates the repeated request work from
// the Apex body. It deliberately uses the same owned org as the HTTP benchmark.
func BenchmarkLightningWireRequestSetup(b *testing.B) {
	handler := lightningWireBenchmarkServer(b)
	b.Run("Runtime", func(b *testing.B) {
		for range b.N {
			if _, err := handler.visualforceRuntime(); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("OrgClone", func(b *testing.B) {
		for range b.N {
			clone := handler.Org.Clone()
			if len(clone.Objects) != len(handler.Org.Objects) {
				b.Fatal("org clone lost objects")
			}
		}
	})
	b.Run("OrgEqual", func(b *testing.B) {
		for range b.N {
			if !orgStateEqual(*handler.Org, *handler.Org) {
				b.Fatal("org comparison changed state")
			}
		}
	})
}
