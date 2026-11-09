@IsTest public class DefaultLoadDmlProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(DefaultLoadDmlProbe.observe())); } }
