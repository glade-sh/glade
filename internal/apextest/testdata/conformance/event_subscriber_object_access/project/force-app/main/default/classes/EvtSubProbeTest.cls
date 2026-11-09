@IsTest public class EvtSubProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(EvtSubProbe.observe())); } }
