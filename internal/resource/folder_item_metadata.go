package resource

import (
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// FolderItemMetadataFiles is the input set for the project runtime snapshot.
func FolderItemMetadataFiles(p project.Project) []string {
	files := append([]string(nil), p.FolderFiles...)
	files = append(files, p.ReportFiles...)
	files = append(files, p.DashboardFiles...)
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status == "loaded" && dep.Project != nil {
			files = append(files, FolderItemMetadataFiles(*dep.Project)...)
		}
	}
	return files
}

// applyProjectFolderItems indexes each discovered file even if its parent
// folder metadata is absent. In that case the folder's label and ID are
// unknown; the local row leaves them null until a folder record is available.
func applyProjectFolderItems(org *storage.OrgState, p project.Project, read func(string) ([]byte, error)) error {
	for _, path := range p.ReportFiles {
		if err := applyProjectReportFile(org, path, p.Namespace, read); err != nil {
			return err
		}
	}
	for _, path := range p.DashboardFiles {
		if err := applyProjectDashboardFile(org, path, p.Namespace, read); err != nil {
			return err
		}
	}
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status == "loaded" && dep.Project != nil {
			if err := applyProjectFolderItems(org, *dep.Project, read); err != nil {
				return err
			}
		}
	}
	return nil
}

func projectFolderValues(org *storage.OrgState, folderType, path string) (storage.Value, storage.Value) {
	folder := filepath.Base(filepath.Dir(path))
	id, ok := projectFolderID(org, folderType, folder)
	if !ok {
		return storage.NullValue(), storage.NullValue()
	}
	record := org.Objects["Folder"].Records[id]
	return storage.IDValue(id), record.Fields["Name"]
}

func applyProjectReportFile(org *storage.OrgState, path, namespace string, read func(string) ([]byte, error)) error {
	data, err := read(path)
	if err != nil {
		return err
	}
	var meta reportXML
	if err = xml.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("load Report metadata %s: %w", path, err)
	}
	_, folderName := projectFolderValues(org, "Report", path)
	storage.EnsureStandardObject(org, "Report")
	object := org.Objects["Report"]
	developerName := metadataNameFromPath(path, ".report-meta.xml", ".report")
	id := metadataRecordIDForIdentity(object, func(record storage.Record) bool {
		return strings.EqualFold(record.Fields["DeveloperName"].String, developerName) && strings.EqualFold(record.Fields["NamespacePrefix"].String, strings.TrimSpace(namespace))
	})
	object.PutRecord(id, storage.Record{ID: id, Object: "Report", Fields: map[string]storage.Value{
		"Id": storage.IDValue(id), "Name": storage.StringValue(firstNonEmpty(strings.TrimSpace(meta.Name), developerName)), "DeveloperName": storage.StringValue(developerName), "FolderName": folderName, "Format": storage.StringValue(strings.TrimSpace(meta.Format)), "NamespacePrefix": nullableNamespace(namespace),
	}})
	org.Objects["Report"] = object
	folderID, _ := projectFolderID(org, "Report", filepath.Base(filepath.Dir(path)))
	definition := storage.ProjectReportMetadata{
		FolderID: folderID,
		ID:       id, Name: firstNonEmpty(strings.TrimSpace(meta.Name), developerName), Description: strings.TrimSpace(meta.Description),
		DeveloperName: developerName, Format: strings.TrimSpace(meta.Format),
		ReportType: strings.TrimSpace(meta.ReportType), Scope: strings.TrimSpace(meta.Scope),
		Supported: len(meta.Other) == 0,
	}
	switch strings.TrimSpace(meta.ShowDetails) {
	case "", "false":
	case "true":
		definition.ShowDetails = true
	default:
		definition.Supported = false
	}
	for _, flag := range []struct {
		source string
		hidden *bool
	}{{meta.ShowGrandTotal, &definition.HideGrandTotal}, {meta.ShowSubTotals, &definition.HideSubTotals}} {
		switch strings.TrimSpace(flag.source) {
		case "", "true":
		case "false":
			*flag.hidden = true
		default:
			definition.Supported = false
		}
	}
	if tf := meta.TimeFrame; tf != nil {
		column := strings.ToUpper(strings.TrimSpace(tf.DateColumn))
		allowed := false
		switch strings.ToLower(definition.ReportType) {
		case "accountlist":
			allowed = column == "DUE_DATE" || column == "CREATED_DATE"
		case "contactlist":
			allowed = column == "LAST_ACTIVITY" || column == "CREATED_DATE"
		case "opportunity":
			allowed = column == "CREATED_DATE"
		}
		if len(tf.Other) != 0 || !strings.EqualFold(strings.TrimSpace(tf.Interval), "INTERVAL_CUSTOM") || !allowed {
			definition.Supported = false
		} else {
			definition.DateColumn = column
			for _, bound := range []struct {
				source *string
				target *string
			}{{tf.StartDate, &definition.StartDate}, {tf.EndDate, &definition.EndDate}} {
				if bound.source == nil {
					continue
				}
				value := strings.TrimSpace(*bound.source)
				if _, err := time.Parse("2006-01-02", value); err != nil {
					definition.Supported = false
				} else {
					*bound.target = value
				}
			}
			if (tf.StartDate != nil || tf.EndDate != nil) && column != "CREATED_DATE" {
				definition.Supported = false
			}
			if definition.StartDate != "" && definition.EndDate != "" && definition.StartDate > definition.EndDate {
				definition.Supported = false
			}
		}
	}
	for _, column := range meta.Columns {
		definition.Columns = append(definition.Columns, strings.TrimSpace(column.Field))
		if len(column.Other) != 0 {
			definition.Supported = false
		}
	}
	for _, grouping := range meta.GroupingsDown {
		definition.GroupingsDown = append(definition.GroupingsDown, strings.TrimSpace(grouping.Field))
		if len(grouping.Other) != 0 || !strings.EqualFold(strings.TrimSpace(grouping.SortOrder), "Asc") {
			definition.Supported = false
		}
	}
	org.Metadata.ProjectReports = append(org.Metadata.ProjectReports, definition)
	return nil
}

func applyProjectDashboardFile(org *storage.OrgState, path, namespace string, read func(string) ([]byte, error)) error {
	data, err := read(path)
	if err != nil {
		return err
	}
	var meta dashboardXML
	if err = xml.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("load Dashboard metadata %s: %w", path, err)
	}
	folderID, folderName := projectFolderValues(org, "Dashboard", path)
	storage.EnsureStandardObject(org, "Dashboard")
	object := org.Objects["Dashboard"]
	developerName := metadataNameFromPath(path, ".dashboard-meta.xml", ".dashboard")
	id := metadataRecordIDForIdentity(object, func(record storage.Record) bool {
		return strings.EqualFold(record.Fields["DeveloperName"].String, developerName) && strings.EqualFold(record.Fields["NamespacePrefix"].String, strings.TrimSpace(namespace))
	})
	object.PutRecord(id, storage.Record{ID: id, Object: "Dashboard", Fields: map[string]storage.Value{
		"Id": storage.IDValue(id), "Title": storage.StringValue(firstNonEmpty(strings.TrimSpace(meta.Title), developerName)), "DeveloperName": storage.StringValue(developerName), "FolderId": folderID, "FolderName": folderName, "Type": storage.StringValue(strings.TrimSpace(meta.Type)), "NamespacePrefix": nullableNamespace(namespace),
	}})
	org.Objects["Dashboard"] = object
	return nil
}
