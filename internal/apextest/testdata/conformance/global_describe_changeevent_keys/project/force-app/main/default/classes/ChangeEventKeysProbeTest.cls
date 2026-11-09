@IsTest public class ChangeEventKeysProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(ChangeEventKeysProbe.observe())); } }
