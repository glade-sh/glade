@IsTest public class ProfileLicenseProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(ProfileLicenseProbe.observe())); } }
