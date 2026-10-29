@IsTest
private class BarConsumerTest {
    @IsTest
    static void compilesAgainstUndeclaredNamespaceTypes() {
        System.assertEquals(null, BarConstructions.make(false));
        System.assertEquals(0, BarConstructions.collect(false).size());
        BarSignatures signatures = new BarSignatures();
        System.assertEquals(null, signatures.relay(null, null, null));
        System.assertEquals(0, signatures.guarded(false));
        Object deferred = null;
        if (deferred != null) { deferred = new bar.Wrapper(); }
        System.assertEquals(null, deferred);
    }
}
