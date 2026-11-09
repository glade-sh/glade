@IsTest public class ApvProbeTest {
    @IsTest static void lockNoProcess() { System.assert(false, 'OBS|' + JSON.serialize(ApvProbe.lockNoProcess())); }
    @IsTest static void lockList() { System.assert(false, 'OBS|' + JSON.serialize(ApvProbe.lockList())); }
    @IsTest static void submitOnly() { System.assert(false, 'OBS|' + JSON.serialize(ApvProbe.submitOnly())); }
    @IsTest static void approve() { System.assert(false, 'OBS|' + JSON.serialize(ApvProbe.decide('Approve'))); }
    @IsTest static void reject() { System.assert(false, 'OBS|' + JSON.serialize(ApvProbe.decide('Reject'))); }
    @IsTest static void recall() { System.assert(false, 'OBS|' + JSON.serialize(ApvProbe.decide('Removed'))); }
    @IsTest static void deploy() { System.assert(false, 'OBS|' + JSON.serialize(ApvProbe.deploy())); }
}
