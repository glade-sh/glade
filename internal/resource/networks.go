package resource

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// ApplyProjectNetworks materializes deployed Network metadata without establishing
// a hosted request context. A supplied reader preserves captured metadata bytes.
func ApplyProjectNetworks(org *storage.OrgState, p project.Project, readers ...func(string) ([]byte, error)) error {
	if org == nil || !org.HasFeature("Communities") || len(p.NetworkFiles) == 0 {
		return nil
	}
	read := os.ReadFile
	if len(readers) > 0 && readers[0] != nil {
		read = readers[0]
	}
	generator := storage.NewStandardIDGenerator()
	generator.Sequences = org.IDSequences
	for _, file := range p.NetworkFiles {
		data, err := read(file)
		if err != nil {
			return err
		}
		var metadata struct {
			XMLName       xml.Name `xml:"Network"`
			Status        string   `xml:"status"`
			URLPathPrefix string   `xml:"urlPathPrefix"`
		}
		if err := xml.Unmarshal(data, &metadata); err != nil {
			return fmt.Errorf("Network metadata %s: %w", file, err)
		}
		name := filepath.Base(file)
		for _, suffix := range []string{".network-meta.xml", ".network"} {
			if len(name) >= len(suffix) && strings.EqualFold(name[len(name)-len(suffix):], suffix) {
				name = name[:len(name)-len(suffix)]
				break
			}
		}
		state := org.Objects["Network"]
		exists := false
		for _, record := range state.Records {
			if strings.EqualFold(record.Fields["Name"].String, name) {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		id, err := generator.Next("Network")
		if err != nil {
			return err
		}
		fields := map[string]storage.Value{
			"Name":                       storage.StringValue(name),
			"Status":                     storage.StringValue(strings.TrimSpace(metadata.Status)),
			"UrlPathPrefix":              storage.StringValue(strings.TrimSpace(metadata.URLPathPrefix)),
			"OptionsGuestChatterEnabled": storage.BooleanValue(false),
		}
		mutable, _ := storage.EnsureMutableObjectRecords(org, "Network")
		if mutable == nil {
			return fmt.Errorf("Network schema is unavailable")
		}
		if mutable.Records == nil {
			mutable.Records = make(map[storage.ID]storage.Record)
		}
		mutable.Records[id] = storage.Record{ID: id, Object: "Network", Fields: fields}
		org.Objects["Network"] = *mutable
	}
	org.IDSequences = generator.Sequences
	return nil
}
