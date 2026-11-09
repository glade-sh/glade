@IsTest public class AuraRedirectProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(AuraRedirectProbe.observe())); } }
