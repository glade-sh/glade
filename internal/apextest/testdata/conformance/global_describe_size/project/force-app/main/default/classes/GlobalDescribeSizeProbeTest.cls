@IsTest public class GlobalDescribeSizeProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(GlobalDescribeSizeProbe.observe())); } }
