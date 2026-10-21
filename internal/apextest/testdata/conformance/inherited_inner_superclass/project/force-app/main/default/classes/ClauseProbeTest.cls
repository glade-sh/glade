@IsTest private class ClauseProbeTest {
    @IsTest static void added() { System.assert(false, 'OBSERVED|added|' + JSON.serialize(new Map<String,Object>{'value'=>ClauseQuery.added()})); }
    @IsTest static void empty() { System.assert(false, 'OBSERVED|empty|' + JSON.serialize(new Map<String,Object>{'value'=>ClauseQuery.empty()})); }
    @IsTest static void populated() { System.assert(false, 'OBSERVED|populated|' + JSON.serialize(new Map<String,Object>{'value'=>ClauseQuery.populated()})); }
    @IsTest static void qualified() { System.assert(false, 'OBSERVED|qualified|' + JSON.serialize(new Map<String,Object>{'value'=>ClauseQuery.qualified()})); }
}
