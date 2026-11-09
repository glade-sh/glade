@IsTest public class QuoteFollowProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(QuoteFollowProbe.observe())); } }
