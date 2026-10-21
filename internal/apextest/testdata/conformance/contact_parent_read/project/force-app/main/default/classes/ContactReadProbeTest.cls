@IsTest public class ContactReadProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(ContactReadProbe.observe())); }
}
