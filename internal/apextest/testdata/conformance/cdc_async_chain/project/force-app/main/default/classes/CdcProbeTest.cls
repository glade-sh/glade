@IsTest public class CdcProbeTest {
    @IsTest static void cdcStop() { System.assert(false, 'OBS|' + JSON.serialize(CdcProbe.cdcStop(false))); }
    @IsTest static void cdcStopEnabled() { System.assert(false, 'OBS|' + JSON.serialize(CdcProbe.cdcStop(true))); }
    @IsTest static void cdcStopOutside() { System.assert(false, 'OBS|' + JSON.serialize(CdcProbe.cdcStopOutside(false))); }
    @IsTest static void cdcStopOutsideEnabled() { System.assert(false, 'OBS|' + JSON.serialize(CdcProbe.cdcStopOutside(true))); }
    @IsTest static void cdcDeliver() { System.assert(false, 'OBS|' + JSON.serialize(CdcProbe.cdcDeliver(false))); }
    @IsTest static void cdcDeliverEnabled() { System.assert(false, 'OBS|' + JSON.serialize(CdcProbe.cdcDeliver(true))); }
    @IsTest static void cdcDeliverNoStop() { System.assert(false, 'OBS|' + JSON.serialize(CdcProbe.cdcDeliverNoStop())); }
    @IsTest static void eventChainDeliver() { System.assert(false, 'OBS|' + JSON.serialize(CdcProbe.eventChainDeliver())); }
    @IsTest static void testChain() { System.assert(false, 'OBS|' + JSON.serialize(CdcProbe.testChain())); }
}
