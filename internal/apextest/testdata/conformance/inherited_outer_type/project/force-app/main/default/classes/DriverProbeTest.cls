@IsTest private class DriverProbeTest {
    @IsTest static void direct() { System.assert(false, 'OBSERVED|direct|' + JSON.serialize(new Map<String,Object>{'value'=>DriverProbe.direct()})); }
    @IsTest static void inherited() { System.assert(false, 'OBSERVED|inherited|' + JSON.serialize(new Map<String,Object>{'value'=>DriverProbe.inherited()})); }
    @IsTest static void qualified() { System.assert(false, 'OBSERVED|qualified|' + JSON.serialize(new Map<String,Object>{'value'=>DriverProbe.qualified()})); }
}
