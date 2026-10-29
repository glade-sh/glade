package apextest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Ownership invariant: project/schema caches retain only immutable preparation.
// A linked base is never executed. Every newMachine owns its clock, random
// stream, context, current user, async queues, limits, statics and org mutations.
// This checks every runner holder (base and derived), both compilers/APIs, two
// seeded orgs, parallel siblings and the row after a dirty failed/cancelled row.
func TestConformanceRunnerPrivateExecutionState(t *testing.T) {
	cache := newConformanceProjectCache(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			files := map[string]string{
				"force-app/main/default/classes/OwnershipWork.cls":  `public class OwnershipWork implements Queueable { public void execute(QueueableContext c) { insert new Account(Name='async-dirty'); } }`,
				"force-app/main/default/classes/OwnershipState.cls": `public class OwnershipState { public static List<String> values=new List<String>{'seed'}; }`,
			}
			index := cache.index(t, api, files)
			request, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, route := range []string{"request", "test"} {
				t.Run(route, func(t *testing.T) {
					var previous *conformanceRunner
					for _, seed := range []string{"seed-a", "seed-b"} {
						t.Run(seed, func(t *testing.T) {
							newOrg := func(t *testing.T) storage.OrgState {
								org := orgFromIndex(index)
								org.APIVersion = api
								fixture := storage.NewFixture()
								fixture.Objects = []storage.FixtureObject{{Name: "Account", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"Name": storage.StringValue(seed)}}}}}
								if err := storage.ApplyFixture(&org, fixture); err != nil {
									t.Fatal(err)
								}
								return org
							}
							org := newOrg(t)
							options := conformanceRunnerOptions{Base: previous, LinkProject: route == "test", Org: &org}
							if route == "request" {
								options.RequestRuntime = &request
							}
							base := newConformanceRunner(t, index, options)
							if previous != nil && base.org == previous.org {
								t.Fatal("different seed org reused the previous fixture template")
							}
							previous = base
							options.Base = base
							derived := newConformanceRunner(t, index, options)
							for _, holder := range []struct {
								name   string
								runner *conformanceRunner
							}{{"base", base}, {"derived", derived}} {
								t.Run(holder.name, func(t *testing.T) {
									for sibling := 0; sibling < 2; sibling++ {
										runConformanceSubtestParallel(t, fmt.Sprint(sibling), func(t *testing.T) {
											freshIndex := newConformanceProjectCache(t).index(t, api, files)
											freshOrg := newOrg(t)
											freshOptions := conformanceRunnerOptions{LinkProject: route == "test", Org: &freshOrg}
											freshRuntime, err := CompileProjectRuntimeForRequestWithSourceDigests(freshIndex, nil)
											if err != nil {
												t.Fatal(err)
											}
											if route == "request" {
												freshOptions.RequestRuntime = &freshRuntime
											}
											fresh := newConformanceRunner(t, freshIndex, freshOptions)
											cleanSource := `System.debug(Datetime.now()); System.debug(Math.random()); System.debug(UserInfo.getUserId()); System.debug(OwnershipState.values); System.debug([SELECT Name FROM Account ORDER BY Name]); System.debug(Limits.getQueueableJobs()); System.debug(Limits.getDmlStatements());`
											clean, err := vm.CompileAnonymousWithOptions(cleanSource, vm.CompileOptions{APIVersion: api})
											if err != nil {
												t.Fatal(err)
											}
											body := `OwnershipState.values.add('dirty'); insert new Account(Name='row-dirty'); Savepoint s=Database.setSavepoint(); insert new Account(Name='rollback-dirty'); Database.rollback(s); System.enqueueJob(new OwnershipWork()); System.debug(Datetime.now()); System.debug(Math.random()); System.debug(UserInfo.getUserId()); Object absent=null; System.debug(absent.toString());`
											if route == "test" {
												body = `System.runAs(new User(Id='005000000000099AAA')) { ` + body + ` }`
											}
											dirtyProgram, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
											if err != nil {
												t.Fatal(err)
											}
											dirty := holder.runner.newMachine()
											if route == "test" {
												dirty.EnableTestContext()
											} else {
												dirty.SetCurrentUser(storage.Record{ID: "005000000000099AAA", Object: "User"})
											}
											clock := dirty.Org.Now()
											dirty.AdvanceDeterministicTime(48 * time.Hour)
											dirty.SetDeterministicRandomState(100 + uint64(sibling))
											dirtyResult, dirtyErr := dirty.Execute(dirtyProgram)
											if dirtyErr == nil {
												t.Fatal("dirty error control did not throw")
											}
											if !strings.Contains(strings.Join(dirtyResult.Debug, "\n"), "005000000000099AAA") {
												t.Fatal("current-user/runAs control did not change user")
											}
											if !dirty.HasPendingAsyncWork() || dirty.DeterministicRandomState() == 100+uint64(sibling) || !dirty.Org.Now().After(clock) {
												t.Fatal("dirty row did not exercise async, random and clock state")
											}
											identity, err := vm.CompileAnonymousWithOptions(`System.debug(UserInfo.getUserId());`, vm.CompileOptions{APIVersion: api})
											if err != nil {
												t.Fatal(err)
											}
											identityResult, err := dirty.Execute(identity)
											wantUser := "005000000000099AAA"
											if route == "test" {
												wantUser = string(storage.DefaultUserID)
											}
											if err != nil || !reflect.DeepEqual(identityResult.Debug, []string{wantUser}) {
												t.Fatalf("user after failed runAs: %v %v", identityResult.Debug, err)
											}
											ctx, cancel := context.WithCancel(context.Background())
											dirty.SetContext(ctx)
											cancel()
											if _, err := dirty.Execute(clean); !errors.Is(err, context.Canceled) {
												t.Fatalf("cancellation control: %v", err)
											}
											// Multiple subsequent transactions must match genuinely fresh runners.
											for row := 0; row < 2; row++ {
												a, b := holder.runner.newMachine(), fresh.newMachine()
												if route == "test" {
													a.EnableTestContext()
													b.EnableTestContext()
												}
												if a.HasPendingAsyncWork() || b.HasPendingAsyncWork() || !a.Org.Now().Equal(b.Org.Now()) || a.DeterministicRandomState() != b.DeterministicRandomState() {
													t.Fatal("execution state leaked to next row")
												}
												if !reflect.DeepEqual(a.Org.IDSequences, b.Org.IDSequences) || !reflect.DeepEqual(a.Org.Transactions, b.Org.Transactions) || !reflect.DeepEqual(a.Org.Objects["Account"].Records, b.Org.Objects["Account"].Records) || !reflect.DeepEqual(a.Org.Objects["Account"].Indexes, b.Org.Objects["Account"].Indexes) {
													t.Fatal("row org records/indexes/sequences/transactions leaked")
												}
												got, gotErr := a.Execute(clean)
												want, wantErr := b.Execute(clean)
												got.Limits.CPUTimeMS = 0
												want.Limits.CPUTimeMS = 0
												// Trace durations and CPU sampling measure real time. Preserve
												// every trace event and all other arguments/positions.
												normalize := func(result vm.Result) []byte {
													for i := range result.Trace {
														result.Trace[i].Duration = 0
														if result.Trace[i].Name == "apex.limits" {
															result.Trace[i].Args["cpuTimeMs"] = 0
														}
													}
													raw, err := json.Marshal(result)
													if err != nil {
														t.Fatal(err)
													}
													return raw
												}
												x, y := normalize(got), normalize(want)
												if gotErr != nil || wantErr != nil || !reflect.DeepEqual(x, y) {
													t.Fatalf("row=%d shared/fresh mismatch: %s / %s errors=%v/%v", row, x, y, gotErr, wantErr)
												}
												joined := strings.Join(got.Debug, "\n")
												if !strings.Contains(joined, seed) || strings.Contains(joined, "dirty") || strings.Contains(joined, "005000000000099AAA") {
													t.Fatalf("dirty org/user escaped row: %s", joined)
												}
											}
											if base.base.HasPendingAsyncWork() || derived.base.HasPendingAsyncWork() {
												t.Fatal("executed a linked base")
											}
										})
									}
								})
							}
						})
					}
				})
			}
		})
	}
}
