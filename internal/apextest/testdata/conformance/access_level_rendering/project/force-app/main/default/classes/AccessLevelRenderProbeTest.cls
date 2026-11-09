@IsTest public class AccessLevelRenderProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(AccessLevelRenderProbe.observe())); } }
