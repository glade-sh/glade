@IsTest public class IdConversionProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(IdConversionProbe.observe())); } }
