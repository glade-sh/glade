@IsTest public class ChangeEventFullProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(ChangeEventFullProbe.observeCompact())); } }
