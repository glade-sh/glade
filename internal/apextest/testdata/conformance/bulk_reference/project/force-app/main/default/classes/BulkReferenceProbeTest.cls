@IsTest public class BulkReferenceProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(BulkReferenceProbe.observe())); }
}
