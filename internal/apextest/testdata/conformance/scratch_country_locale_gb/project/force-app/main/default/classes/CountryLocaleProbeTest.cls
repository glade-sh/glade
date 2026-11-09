@IsTest public class CountryLocaleProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(CountryLocaleProbe.observe())); } }
