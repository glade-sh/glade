@IsTest public class DmlNotAllowedProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(DmlNotAllowedProbe.observe())); } }
