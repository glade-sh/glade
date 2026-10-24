package vm

import (
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

type runtimeGenerationMetadataCaches struct {
	childRel                 *childRelationshipCache
	childRelationshipLookup  *childRelationshipLookupCache
	jsonChildRelType         *jsonChildRelTypeLookupCache
	sObjectFieldAlias        *sObjectFieldAliasLookupCache
	fieldResolve             *fieldResolveLookupCache
	soqlExecution            *soql.ExecutionCache
	loadedChildRelationship  *loadedChildRelationshipLookupCache
	lazyChildRelationship    *lazyChildRelationshipLookupCache
	summarySideEffectObjects *summarySideEffectObjectCache
	describeShare            *describeShareCache
}

func runtimeGenerationCachesForOrg(org *storage.OrgState, stamp string) *runtimeGenerationMetadataCaches {
	return org.RuntimeGenerationCacheValue("vm:metadata-indexes", func() any {
		return &runtimeGenerationMetadataCaches{
			childRel:                 newChildRelationshipCache(),
			childRelationshipLookup:  newChildRelationshipLookupCache(),
			jsonChildRelType:         newJSONChildRelTypeLookupCache(),
			sObjectFieldAlias:        newSObjectFieldAliasLookupCache(),
			fieldResolve:             newFieldResolveLookupCache(),
			soqlExecution:            soql.NewExecutionCache(),
			loadedChildRelationship:  newLoadedChildRelationshipLookupCache(),
			lazyChildRelationship:    newLazyChildRelationshipLookupCache(),
			summarySideEffectObjects: newSummarySideEffectObjectCache(stamp),
			describeShare:            newDescribeShareCache(stamp),
		}
	}).(*runtimeGenerationMetadataCaches)
}

func summaryRelationCacheForOrg(org *storage.OrgState) *dml.SummaryRelationCache {
	return org.RuntimeGenerationCacheValue("dml:summary-relations", func() any {
		return dml.NewSummaryRelationCache()
	}).(*dml.SummaryRelationCache)
}
