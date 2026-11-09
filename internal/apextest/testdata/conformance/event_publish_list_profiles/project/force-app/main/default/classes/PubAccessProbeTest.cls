@IsTest public class PubAccessProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(PubAccessProbe.observe())); } }
