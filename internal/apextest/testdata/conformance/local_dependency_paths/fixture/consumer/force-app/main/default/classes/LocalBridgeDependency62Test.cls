@IsTest
private class LocalBridgeDependency62Test {
    @IsTest
    static void callsLocalDependency() {
        System.assertEquals(null, LocalBridgeSupport62.echo(null));
        System.assertEquals('bridge-ready', LocalBridgeSupport62.state());
    }
}
