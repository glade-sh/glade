@IsTest
private class AssetOwnerProbeTest {
    @IsTest
    static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(AssetOwnerProbe.observe()));
    }
}
