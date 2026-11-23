@IsTest
private class EvtProbeTest {
    @IsTest static void evt() { System.assert(false, 'OUT:' + EvtProbe.evt('t')); }
    @IsTest static void evt2() { System.assert(false, 'OUT:' + EvtProbe.evt2('t')); }
    @IsTest static void evt4() { System.assert(false, 'OUT:' + EvtProbe.evt4('t')); }
    @IsTest static void evt5() { System.assert(false, 'OUT:' + EvtProbe.evt5('t')); }
    @IsTest static void bad() { System.assert(false, 'OUT:' + EvtProbe.bad('t')); }
    @IsTest static void good() { System.assert(false, 'OUT:' + EvtProbe.good('t')); }
    @IsTest static void badId() { System.assert(false, 'OUT:' + EvtProbe.badId('t')); }
    @IsTest static void uncaught() { System.assert(false, 'OUT:' + EvtProbe.uncaught('t')); }
    @IsTest static void dsc() { System.assert(false, 'OUT:' + EvtProbe.dsc('t')); }
}
