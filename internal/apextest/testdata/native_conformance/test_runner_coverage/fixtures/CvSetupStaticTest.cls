@IsTest
private class CvSetupStaticTest {
    @IsTest static void readsState() {
        System.assertEquals(5, CvSetupStatic.state);
    }
}
