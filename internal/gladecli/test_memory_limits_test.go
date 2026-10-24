package gladecli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/testdaemon"
)

func TestApplyTestMemoryLimitsRoutes(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		for _, aggressive := range []bool{false, true} {
			t.Run(fmt.Sprintf("explicit_limit_%t/aggressive_%t", explicit, aggressive), func(t *testing.T) {
				t.Setenv("GOMEMLIMIT", "")
				wantLimit := int64(4 << 30)
				if explicit {
					t.Setenv("GOMEMLIMIT", "2GiB")
					wantLimit = 2 << 30
				}
				// The Go runtime reads GOMEMLIMIT at startup, before this helper.
				oldLimit := debug.SetMemoryLimit(2 << 30)
				oldGC := debug.SetGCPercent(73)
				t.Cleanup(func() {
					debug.SetGCPercent(oldGC)
					debug.SetMemoryLimit(oldLimit)
				})
				applyTestMemoryLimits(aggressive)
				wantGC := 73
				if aggressive {
					wantGC = -1
				}
				if got := debug.SetGCPercent(wantGC); got != wantGC {
					t.Errorf("GC percent = %d, want %d", got, wantGC)
				}
				if got := debug.SetMemoryLimit(-1); got != wantLimit {
					t.Errorf("memory limit = %d, want %d", got, wantLimit)
				}
			})
		}
	}
}

func TestRunTestMemoryLimitsProjectRoutes(t *testing.T) {
	for _, policy := range testMemoryLimitPolicies {
		t.Run(policy.name, func(t *testing.T) {
			for _, api := range []string{"62.0", "67.0"} {
				for _, route := range []struct {
					name   string
					args   []string
					server bool
				}{
					{"local_serial", []string{"--no-cache", "--parallelism", "1"}, false},
					{"local_parallel", []string{"--no-cache", "--parallelism", "2"}, false},
					{"local_cached", nil, false},
					{"daemon", []string{"--daemon"}, false},
					{"watch_once", []string{"--watch-once"}, false},
					{"daemon_watch_once", []string{"--daemon", "--watch-once"}, false},
					{"server_auto", nil, true},
					{"server_connect", []string{"--connect"}, true},
				} {
					t.Run(api+"/"+route.name, func(t *testing.T) {
						t.Setenv("GOMEMLIMIT", policy.env)
						oldLimit := debug.SetMemoryLimit(policy.limit)
						oldGC := debug.SetGCPercent(100)
						t.Cleanup(func() {
							debug.SetGCPercent(oldGC)
							debug.SetMemoryLimit(oldLimit)
						})
						root := t.TempDir()
						writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
						classes := filepath.Join(root, "force-app", "main", "default", "classes")
						writeTestFile(t, filepath.Join(classes, "AllocationTest.cls"), `@IsTest private class AllocationTest {
			@IsTest static void allocates() {
				List<String> values = new List<String>();
				for (Integer i = 0; i < 100; i++) { values.add('entry-' + i); }
				System.assertEquals(100, values.size());
			}
		}`)
						writeTestFile(t, filepath.Join(classes, "AllocationTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
						ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
						defer cancel()
						args := []string{"--project", root, "--gc-aggressive", "--no-progress"}
						if route.server {
							socket := testdaemon.ServeSocketPath(root)
							server, err := testdaemon.NewServer(testdaemon.ServerConfig{Root: root, Socket: socket, Warm: true})
							if err != nil {
								t.Fatal(err)
							}
							done := make(chan error, 1)
							go func() { done <- server.ListenAndServe(ctx, io.Discard) }()
							t.Cleanup(func() {
								cancel()
								select {
								case <-done:
								case <-time.After(5 * time.Second):
									t.Error("test server did not stop")
								}
							})
							waitForServer(t, ctx, socket)
						} else {
							args = append(args, "--no-serve")
						}
						args = append(args, route.args...)
						var stdout, stderr bytes.Buffer
						run, err := runTest(ctx, args, &stdout, &stderr)
						if err != nil {
							t.Fatalf("run: %v; stdout=%s stderr=%s", err, &stdout, &stderr)
						}
						if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
							t.Fatalf("project summary = %#v, want one passing test", got)
						}
						if got := debug.SetGCPercent(policy.gc); got != policy.gc {
							t.Errorf("project route GC percent = %d, want %d", got, policy.gc)
						}
						if got := debug.SetMemoryLimit(-1); got != policy.limit {
							t.Errorf("project route memory limit = %d, want %d", got, policy.limit)
						}
					})
				}
			}
		})
	}
}

func BenchmarkTestMemoryLimitAllocation(b *testing.B) {
	for _, percent := range []int{50, 100, 200, -1} {
		b.Run(fmt.Sprintf("GOGC_%d", percent), func(b *testing.B) {
			oldLimit := debug.SetMemoryLimit(256 << 20)
			oldGC := debug.SetGCPercent(percent)
			defer debug.SetMemoryLimit(oldLimit)
			defer debug.SetGCPercent(oldGC)
			const chunk = 32 << 10
			live := make([][]byte, 1024) // Retain 32 MiB while allocating 64 MiB per iteration.
			for i := range live {
				live[i] = make([]byte, chunk)
			}
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for j := 0; j < 2048; j++ {
					data := make([]byte, chunk)
					data[0], data[len(data)-1] = byte(j), byte(i)
					live[j%len(live)] = data
				}
			}
			b.StopTimer()
			runtime.ReadMemStats(&after)
			runtime.KeepAlive(live)
			b.ReportMetric(float64(after.NumGC-before.NumGC)/float64(b.N), "GC/op")
			b.ReportMetric(float64(after.PauseTotalNs-before.PauseTotalNs)/float64(b.N), "pause-ns/op")
		})
	}
}
