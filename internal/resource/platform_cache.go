package resource

import (
	"encoding/xml"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

type platformCachePartitionXML struct {
	XMLName            xml.Name                        `xml:"PlatformCachePartition"`
	FullName           string                          `xml:"fullName"`
	MasterLabel        string                          `xml:"masterLabel"`
	Description        string                          `xml:"description"`
	IsDefaultPartition bool                            `xml:"isDefaultPartition"`
	Types              []platformCachePartitionTypeXML `xml:"platformCachePartitionTypes"`
}

type platformCachePartitionTypeXML struct {
	CacheType                  string `xml:"cacheType"`
	AllocatedCapacity          *int64 `xml:"allocatedCapacity"`
	AllocatedPartnerCapacity   *int64 `xml:"allocatedPartnerCapacity"`
	AllocatedPurchasedCapacity *int64 `xml:"allocatedPurchasedCapacity"`
	AllocatedTrialCapacity     *int64 `xml:"allocatedTrialCapacity"`
}

// Partition metadata is materialized as the standard setup objects consumed by
// Cache.Org and Cache.Session. All project runtime entry points use ApplyProject.
func applyPlatformCachePartitions(org *storage.OrgState, p project.Project) error {
	if org == nil {
		return nil
	}
	// The native platform rejects a second local default and leaves both the
	// existing setup and routing unchanged. Stage only the two cache objects.
	staged := *org
	staged.Objects = maps.Clone(org.Objects)
	for _, name := range []string{"PlatformCachePartition", "PlatformCachePartitionType"} {
		if object, ok := org.Objects[name]; ok {
			staged.Objects[name] = object.Clone()
		}
	}
	if err := loadPlatformCachePartitions(&staged, p); err != nil {
		return err
	}
	defaults := map[string]int{}
	for _, record := range staged.Objects["PlatformCachePartition"].Records {
		if !record.Fields["IsDefaultPartition"].Boolean {
			continue
		}
		namespace := strings.ToLower(record.Fields["NamespacePrefix"].String)
		defaults[namespace]++
		if defaults[namespace] > 1 {
			return errors.New("Only one default partition is allowed per organization")
		}
	}
	// Mixed managed/local defaults are an uncaptured boundary. Keep their
	// metadata separate; the cache runtime deterministically prefers local.
	for _, name := range []string{"PlatformCachePartition", "PlatformCachePartitionType"} {
		if object, ok := staged.Objects[name]; ok {
			org.Objects[name] = object
		}
	}
	org.ClearRuntimeSchemaStamp()
	return nil
}

func loadPlatformCachePartitions(org *storage.OrgState, p project.Project) error {
	paths := append([]string(nil), p.CachePartitionFiles...)
	sort.Strings(paths)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("load PlatformCachePartition metadata %s: %w", path, err)
		}
		var raw platformCachePartitionXML
		if err := xml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("load PlatformCachePartition metadata %s: %w", path, err)
		}
		name := strings.TrimSpace(raw.FullName)
		if name == "" {
			name = trimKnownSuffix(filepath.Base(path), ".cachePartition-meta.xml")
		}
		namespace := namespaceremap.ApplyNamespace(p.NamespaceRemaps, p.Namespace)
		storage.EnsureStandardObject(org, "PlatformCachePartition")
		storage.EnsureStandardObject(org, "PlatformCachePartitionType")
		partitions, _ := storage.EnsureMutableObjectRecords(org, "PlatformCachePartition")
		types, _ := storage.EnsureMutableObjectRecords(org, "PlatformCachePartitionType")
		id := metadataRecordIDForIdentity(*partitions, func(record storage.Record) bool {
			return strings.EqualFold(record.Fields["DeveloperName"].String, name) && strings.EqualFold(record.Fields["NamespacePrefix"].String, namespace)
		})
		if raw.IsDefaultPartition && namespace == "" {
			// Preserve the pre-existing local feature fallback: source metadata
			// replaces that synthetic, capacity-less seed, not an authored default.
			seedID := storage.ID("0Px000000000001")
			seed, found := partitions.Records[seedID]
			if found && seedID != id && seed.Fields["DeveloperName"].String == "default" && seed.Fields["NamespacePrefix"].String == "" && seed.Fields["MasterLabel"].String == "default" {
				configured := false
				for _, allocation := range types.Records {
					if allocation.Fields["PlatformCachePartitionId"].ID == seedID {
						configured = true
						break
					}
				}
				if !configured {
					seed = seed.Clone()
					seed.Fields["IsDefaultPartition"] = storage.BooleanValue(false)
					partitions.Records[seedID] = seed
				}
			}
		}
		partitions.Records[id] = storage.Record{ID: id, Object: "PlatformCachePartition", Fields: map[string]storage.Value{
			"Id":                 storage.IDValue(id),
			"DeveloperName":      storage.StringValue(name),
			"NamespacePrefix":    storage.StringValue(namespace),
			"MasterLabel":        storage.StringValue(raw.MasterLabel),
			"Description":        storage.StringValue(raw.Description),
			"IsDefaultPartition": storage.BooleanValue(raw.IsDefaultPartition),
		}}
		for _, allocation := range raw.Types {
			typeID := metadataRecordIDForIdentity(*types, func(record storage.Record) bool {
				return record.Fields["PlatformCachePartitionId"].ID == id && strings.EqualFold(record.Fields["CacheType"].String, allocation.CacheType)
			})
			fields := map[string]storage.Value{
				"Id":                       storage.IDValue(typeID),
				"PlatformCachePartitionId": storage.IDValue(id),
				"CacheType":                storage.StringValue(allocation.CacheType),
			}
			for field, value := range map[string]*int64{
				"AllocatedCapacity":          allocation.AllocatedCapacity,
				"AllocatedPartnerCapacity":   allocation.AllocatedPartnerCapacity,
				"AllocatedPurchasedCapacity": allocation.AllocatedPurchasedCapacity,
				"AllocatedTrialCapacity":     allocation.AllocatedTrialCapacity,
			} {
				if value != nil {
					fields[field] = storage.IntegerValue(*value)
				}
			}
			types.Records[typeID] = storage.Record{ID: typeID, Object: "PlatformCachePartitionType", Fields: fields}
		}
		org.Objects["PlatformCachePartition"] = *partitions
		org.Objects["PlatformCachePartitionType"] = *types
	}
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status == "loaded" && dep.Project != nil {
			if err := loadPlatformCachePartitions(org, *dep.Project); err != nil {
				return err
			}
		}
	}
	return nil
}
