@IsTest public class UnlistedProfileProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(UnlistedProfileProbe.observe())); } }
