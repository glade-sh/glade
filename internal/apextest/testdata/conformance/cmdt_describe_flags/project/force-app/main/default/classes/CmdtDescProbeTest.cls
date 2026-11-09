@IsTest public class CmdtDescProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(CmdtDescProbe.observe())); } }
