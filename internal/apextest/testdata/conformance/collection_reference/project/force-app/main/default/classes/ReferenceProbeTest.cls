@IsTest public class ReferenceProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(ReferenceProbe.observe())); }
}
