package playground

import (
	"encoding/json"
	"sort"

	"github.com/open-aer/oaer/internal/storage"
)

func diffOrg(before, after storage.OrgState) []OrgDiff {
	names := make(map[string]bool)
	for name := range before.Objects {
		names[name] = true
	}
	for name := range after.Objects {
		names[name] = true
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)

	var out []OrgDiff
	for _, name := range ordered {
		b := before.Objects[name].Records
		a := after.Objects[name].Records
		item := OrgDiff{Object: name}
		for id, afterRecord := range a {
			beforeRecord, ok := b[id]
			switch {
			case !ok:
				item.Inserted++
				item.InsertedIDs = append(item.InsertedIDs, string(id))
			case recordJSON(beforeRecord) != recordJSON(afterRecord):
				item.Updated++
				item.UpdatedIDs = append(item.UpdatedIDs, string(id))
			}
		}
		for id := range b {
			if _, ok := a[id]; !ok {
				item.Deleted++
				item.DeletedIDs = append(item.DeletedIDs, string(id))
			}
		}
		if item.Inserted != 0 || item.Updated != 0 || item.Deleted != 0 {
			sort.Strings(item.InsertedIDs)
			sort.Strings(item.UpdatedIDs)
			sort.Strings(item.DeletedIDs)
			out = append(out, item)
		}
	}
	return out
}

func recordJSON(record storage.Record) string {
	data, _ := json.Marshal(record)
	return string(data)
}
