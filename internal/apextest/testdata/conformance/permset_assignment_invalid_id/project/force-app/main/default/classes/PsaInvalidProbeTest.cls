@IsTest public class PsaInvalidProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(PsaInvalidProbe.observe())); } }
