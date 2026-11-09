@IsTest public class RtPresenceProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(RtPresenceProbe.observe())); } }
