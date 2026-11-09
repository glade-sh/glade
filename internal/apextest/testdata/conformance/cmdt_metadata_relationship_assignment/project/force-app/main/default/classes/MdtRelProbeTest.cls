@IsTest public class MdtRelProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(MdtRelProbe.observe())); } }
