@IsTest public class QueryPageBoundaryProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(QueryPageBoundaryProbe.observe()));
    }
}
