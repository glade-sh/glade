@IsTest public class ListAliasProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(ListAliasProbe.observe())); } }
