@IsTest public class TextIdBindProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(TextIdBindProbe.observe())); } }
