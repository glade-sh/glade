@IsTest private class NotAndExplicitTest {
    @IsTest private static void observe() {
        System.assert(false, 'NOTAND|'+JSON.serialize(NotAndQueries.observe()));
    }
}
