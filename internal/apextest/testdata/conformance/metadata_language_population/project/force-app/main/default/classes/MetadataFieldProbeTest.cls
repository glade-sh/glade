@IsTest private class MetadataFieldProbeTest {
    @IsTest static void language() { System.assert(false, 'OBSERVED|language|' + JSON.serialize(new Map<String,Object>{'value'=>MetadataFieldProbe.language()})); }
    @IsTest static void populated() { System.assert(false, 'OBSERVED|populated|' + JSON.serialize(new Map<String,Object>{'value'=>MetadataFieldProbe.populated()})); }
    @IsTest static void queryFields() { System.assert(false, 'OBSERVED|queryFields|' + JSON.serialize(new Map<String,Object>{'value'=>MetadataFieldProbe.queryFields()})); }
    @IsTest static void allPopulated() { System.assert(false, 'OBSERVED|allPopulated|' + JSON.serialize(new Map<String,Object>{'value'=>MetadataFieldProbe.allPopulated()})); }
    @IsTest static void constructed() { System.assert(false, 'OBSERVED|constructed|' + JSON.serialize(new Map<String,Object>{'value'=>MetadataFieldProbe.constructed()})); }
}
