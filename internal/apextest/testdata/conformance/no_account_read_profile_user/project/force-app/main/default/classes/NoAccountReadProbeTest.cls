@IsTest public class NoAccountReadProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(NoAccountReadProbe.observe())); } }
