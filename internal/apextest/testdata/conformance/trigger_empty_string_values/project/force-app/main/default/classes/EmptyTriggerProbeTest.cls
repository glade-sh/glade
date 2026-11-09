@IsTest public class EmptyTriggerProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(EmptyTriggerProbe.observe())); } }
