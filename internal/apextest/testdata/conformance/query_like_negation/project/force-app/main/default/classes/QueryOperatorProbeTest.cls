@IsTest private class QueryOperatorProbeTest {
    @IsTest static void postfix() { System.assert(false, 'OBSERVED|postfix|' + JSON.serialize(new Map<String,Object>{'value'=>QueryOperatorProbe.postfix()})); }
    @IsTest static void lowercase() { System.assert(false, 'OBSERVED|lowercase|' + JSON.serialize(new Map<String,Object>{'value'=>QueryOperatorProbe.lowercase()})); }
    @IsTest static void prefix() { System.assert(false, 'OBSERVED|prefix|' + JSON.serialize(new Map<String,Object>{'value'=>QueryOperatorProbe.prefix()})); }
    @IsTest static void notIn() { System.assert(false, 'OBSERVED|notIn|' + JSON.serialize(new Map<String,Object>{'value'=>QueryOperatorProbe.notIn()})); }
}
