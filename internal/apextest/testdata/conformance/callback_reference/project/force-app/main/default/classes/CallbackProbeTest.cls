@IsTest public class CallbackProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(CallbackProbe.observe())); }
}
