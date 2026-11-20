package dml

import (
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// refreshActivityAccountsFor bumps a package-level scan counter, and parallel
// conformance subtests reach that counter at the same time: each subtest owns
// its engine and org, but package state is shared. Two engines refreshing
// their own orgs concurrently must stay race-free, and each refresh must
// still recompute the stored activity accounts it owes.
func TestActivityRefreshScansConcurrentEngines(t *testing.T) {
	const engines = 2
	const scans = 50
	type engineOrg struct {
		engine    Engine
		accountID storage.ID
		taskID    storage.ID
	}
	owned := make([]engineOrg, engines)
	for i := range owned {
		org := testOrg()
		for _, name := range []string{"Account", "Task"} {
			storage.EnsureStandardObject(&org, name)
		}
		engine := NewEngine(&org)
		accounts := engine.Insert([]storage.Record{
			{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("A")}},
		})
		if !accounts[0].Success {
			t.Fatalf("insert account %d: %#v", i, accounts[0])
		}
		tasks := engine.Insert([]storage.Record{
			{Object: "Task", Fields: map[string]storage.Value{"Subject": storage.StringValue("t"), "WhatId": storage.IDValue(accounts[0].ID)}},
		})
		if !tasks[0].Success {
			t.Fatalf("insert task %d: %#v", i, tasks[0])
		}
		record := org.Objects["Task"].Records[tasks[0].ID]
		delete(record.Fields, "AccountId")
		org.Objects["Task"].Records[tasks[0].ID] = record
		owned[i] = engineOrg{engine: engine, accountID: accounts[0].ID, taskID: tasks[0].ID}
	}

	start := make(chan struct{})
	var done sync.WaitGroup
	for i := range owned {
		done.Add(1)
		go func(own engineOrg) {
			defer done.Done()
			parents := map[storage.ID]struct{}{activityParentKey(own.accountID): {}}
			<-start
			for scan := 0; scan < scans; scan++ {
				own.engine.refreshActivityAccountsFor(parents)
			}
		}(owned[i])
	}
	close(start)
	done.Wait()

	for i, own := range owned {
		record := own.engine.Org.Objects["Task"].Records[own.taskID]
		if got := idFromStorageValue(record.Fields["AccountId"]); got != own.accountID {
			t.Fatalf("engine %d task %s AccountId = %q, want %q: the concurrent scan must still recompute stored activity accounts", i, own.taskID, got, own.accountID)
		}
	}
}
