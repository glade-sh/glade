package resource

import (
	"encoding/xml"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

type queueXML struct {
	XMLName                xml.Name `xml:"Queue"`
	FullName               string   `xml:"fullName"`
	Name                   string   `xml:"name"`
	Email                  string   `xml:"email"`
	DoesSendEmailToMembers bool     `xml:"doesSendEmailToMembers"`
	SObjects               []struct {
		SObjectType string `xml:"sobjectType"`
	} `xml:"queueSobject"`
}

// ApplyProjectQueues projects declared queues as Group rows of Type Queue with
// their QueueSobject rows. Scratch-org rows (API 62): DeveloperName is the file
// name, Name the label, DoesIncludeBosses true, RelatedId null, Email and
// DoesSendEmailToMembers as declared. Members are not synthesized.
func ApplyProjectQueues(org *storage.OrgState, p project.Project) error {
	if org == nil {
		return nil
	}
	paths := projectQueueFiles(p)
	if len(paths) == 0 {
		return nil
	}
	sort.Strings(paths)
	queues := make([]queueXML, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("load Queue metadata %s: %w", path, err)
		}
		var queue queueXML
		if err := xml.Unmarshal(data, &queue); err != nil {
			return fmt.Errorf("load Queue metadata %s: %w", path, err)
		}
		queue.FullName = strings.TrimSpace(queue.FullName)
		if queue.FullName == "" {
			queue.FullName = trimKnownSuffix(trimKnownSuffix(filepath.Base(path), ".queue-meta.xml"), ".queue")
		}
		queues = append(queues, queue)
	}
	staged := *org
	staged.Objects = maps.Clone(org.Objects)
	storage.EnsureStandardObject(&staged, "Group")
	storage.EnsureStandardObject(&staged, "QueueSobject")
	groups := staged.Objects["Group"].Clone()
	links := staged.Objects["QueueSobject"].Clone()
	byName := make(map[string]storage.ID, len(groups.Records))
	for id, r := range groups.Records {
		if r.Fields["Type"].String != "Queue" {
			continue
		}
		name := strings.ToLower(r.Fields["DeveloperName"].String)
		if previous := byName[name]; previous == "" || id < previous {
			byName[name] = id
		}
	}
	generator := storage.NewStandardIDGenerator()
	generator.Sequences["Group"] = org.IDSequences["Group"]
	generator.Sequences["QueueSobject"] = org.IDSequences["QueueSobject"]
	next := func(object string, occupied map[storage.ID]storage.Record) (storage.ID, error) {
		for {
			id, err := generator.Next(object)
			if err != nil {
				return "", err
			}
			if _, _, taken := storage.LookupRecordByID(occupied, id); !taken {
				return id, nil
			}
		}
	}
	for _, queue := range queues {
		key := strings.ToLower(queue.FullName)
		id, exists := byName[key]
		var record storage.Record
		if exists {
			record = groups.Records[id].Clone()
		} else {
			var err error
			if id, err = next("Group", groups.Records); err != nil {
				return err
			}
			byName[key] = id
			record = storage.Record{ID: id, Object: "Group", Fields: make(map[string]storage.Value)}
		}
		record.Fields["Id"] = storage.IDValue(id)
		record.Fields["DeveloperName"] = storage.StringValue(queue.FullName)
		record.Fields["Name"] = storage.StringValue(queue.Name)
		record.Fields["Type"] = storage.StringValue("Queue")
		record.Fields["RelatedId"] = storage.NullValue()
		record.Fields["DoesIncludeBosses"] = storage.BooleanValue(true)
		record.Fields["DoesSendEmailToMembers"] = storage.BooleanValue(queue.DoesSendEmailToMembers)
		if strings.TrimSpace(queue.Email) != "" {
			record.Fields["Email"] = storage.StringValue(strings.TrimSpace(queue.Email))
		} else {
			record.Fields["Email"] = storage.NullValue()
		}
		groups.Records[id] = record
		linked := make(map[string]bool)
		for _, existing := range links.Records {
			if storage.IDsEqual(existing.Fields["QueueId"].ID, id) {
				linked[strings.ToLower(existing.Fields["SobjectType"].String)] = true
			}
		}
		for _, sobject := range queue.SObjects {
			objectName := strings.TrimSpace(sobject.SObjectType)
			if objectName == "" || linked[strings.ToLower(objectName)] {
				continue
			}
			linkID, err := next("QueueSobject", links.Records)
			if err != nil {
				return err
			}
			links.Records[linkID] = storage.Record{ID: linkID, Object: "QueueSobject", Fields: map[string]storage.Value{
				"Id":          storage.IDValue(linkID),
				"QueueId":     storage.IDValue(id),
				"SobjectType": storage.StringValue(objectName),
			}}
			linked[strings.ToLower(objectName)] = true
		}
	}
	if org.Objects == nil {
		org.Objects = make(map[string]storage.ObjectState)
	}
	org.Objects["Group"] = groups
	org.Objects["QueueSobject"] = links
	if org.IDSequences == nil {
		org.IDSequences = make(map[string]uint64)
	}
	org.IDSequences["Group"] = generator.Sequences["Group"]
	org.IDSequences["QueueSobject"] = generator.Sequences["QueueSobject"]
	org.ClearRuntimeSchemaStamp()
	return nil
}

func projectQueueFiles(p project.Project) []string {
	files := append([]string(nil), p.QueueFiles...)
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status == "loaded" && dep.Project != nil {
			files = append(files, projectQueueFiles(*dep.Project)...)
		}
	}
	return files
}
