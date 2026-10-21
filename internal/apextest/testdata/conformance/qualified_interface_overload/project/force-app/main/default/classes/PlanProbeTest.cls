@IsTest private class PlanProbeTest {
    @IsTest static void concrete() { System.assert(false, 'OBSERVED|concrete|' + JSON.serialize(new Map<String,Object>{'value'=>PlanProbe.concrete()})); }
    @IsTest static void inherited() { System.assert(false, 'OBSERVED|inherited|' + JSON.serialize(new Map<String,Object>{'value'=>PlanProbe.inherited()})); }
    @IsTest static void throughInterface() { System.assert(false, 'OBSERVED|throughInterface|' + JSON.serialize(new Map<String,Object>{'value'=>PlanProbe.throughInterface()})); }
}
