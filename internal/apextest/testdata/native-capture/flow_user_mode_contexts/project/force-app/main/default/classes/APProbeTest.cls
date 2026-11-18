@IsTest public class APProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(APProbe.observe())); } }
