@IsTest public class StubProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(StubProbe.observe())); } }
