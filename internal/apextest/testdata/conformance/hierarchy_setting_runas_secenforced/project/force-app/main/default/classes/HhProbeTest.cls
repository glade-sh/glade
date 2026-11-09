@IsTest public class HhProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(HhProbe.observe())); } }
