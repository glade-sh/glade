@IsTest public class RenderWhatProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(RenderWhatProbe.observe())); } }
