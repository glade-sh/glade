@IsTest public class RunAsUnsavedProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(RunAsUnsavedProbe.observe())); } }
