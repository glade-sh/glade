@IsTest public class FlowEmailLogProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(FlowEmailLogProbe.observe())); } }
