@IsTest public class QuoteTotalsProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(QuoteTotalsProbe.observe())); } }
