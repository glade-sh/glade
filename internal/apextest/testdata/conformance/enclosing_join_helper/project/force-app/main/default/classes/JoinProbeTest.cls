@IsTest private class JoinProbeTest {
    @IsTest static void fieldCall() { System.assert(false, 'OBSERVED|fieldCall|' + JSON.serialize(new Map<String,Object>{'value'=>TextJoiner.fieldCall()})); }
    @IsTest static void localCall() { System.assert(false, 'OBSERVED|localCall|' + JSON.serialize(new Map<String,Object>{'value'=>TextJoiner.localCall()})); }
}
