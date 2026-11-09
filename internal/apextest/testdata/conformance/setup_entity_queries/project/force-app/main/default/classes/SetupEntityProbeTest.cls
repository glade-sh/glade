@IsTest public class SetupEntityProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(SetupEntityProbe.observe())); } }
