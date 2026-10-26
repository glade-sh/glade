@IsTest
public class AccessGroupProbeTest {
    @IsTest
    static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(AccessGroupProbe.observe()));
    }
}
