@IsTest public class ListRunAsProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(ListRunAsProbe.observe())); } }
