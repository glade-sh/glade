@IsTest
private class SrtRouteTest {
    @IsTest
    static void ascSlice() {
        SrtProbe.seed();
        System.assert(false, 'a0|' + SrtProbe.slice('ASC', 0, 60));
    }
}
