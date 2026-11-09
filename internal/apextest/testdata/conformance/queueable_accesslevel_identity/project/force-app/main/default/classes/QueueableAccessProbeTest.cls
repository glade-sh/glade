@IsTest public class QueueableAccessProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(QueueableAccessProbe.observe())); } }
