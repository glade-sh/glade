@IsTest public class JsonUserProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(JsonUserProbe.observe())); } }
