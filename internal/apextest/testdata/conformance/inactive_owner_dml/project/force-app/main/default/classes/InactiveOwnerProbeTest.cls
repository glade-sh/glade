@IsTest public class InactiveOwnerProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(InactiveOwnerProbe.observe())); } }
