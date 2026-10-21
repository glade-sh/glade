@IsTest public class QualifiedPageProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(QualifiedPageProbe.observe())); } }
