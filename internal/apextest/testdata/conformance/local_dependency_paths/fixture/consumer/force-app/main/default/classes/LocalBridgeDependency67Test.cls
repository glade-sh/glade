@IsTest
private class LocalBridgeDependency67Test {
    @IsTest
    static void callsLocalDependency() {
        System.assertEquals(null, LocalBridgeSupport67.echo(null));
        System.assertEquals('bridge-ready', LocalBridgeSupport67.state());
    }
}
