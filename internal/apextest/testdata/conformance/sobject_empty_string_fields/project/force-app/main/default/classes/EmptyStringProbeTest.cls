@IsTest public class EmptyStringProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(EmptyStringProbe.observe())); } }
