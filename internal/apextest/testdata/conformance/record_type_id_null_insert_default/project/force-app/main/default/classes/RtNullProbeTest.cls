@IsTest public class RtNullProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(RtNullProbe.observe())); } }
