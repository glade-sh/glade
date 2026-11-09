@IsTest public class UserPsaProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(UserPsaProbe.observe())); } }
