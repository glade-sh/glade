@IsTest public class NullCompareProbeTest { @IsTest static void observe() { throw new NullCompareProbe.ObservationException('PROBE|' + JSON.serialize(NullCompareProbe.observe())); } }
