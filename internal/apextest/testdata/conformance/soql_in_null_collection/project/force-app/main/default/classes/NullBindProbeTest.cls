@IsTest public class NullBindProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(NullBindProbe.observe())); } }
