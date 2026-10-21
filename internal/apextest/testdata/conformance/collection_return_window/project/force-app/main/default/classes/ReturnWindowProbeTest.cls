@IsTest public class ReturnWindowProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(ReturnWindowProbe.observe())); }
}
