@isTest
private class FailingTest {
  @isTest static void fails() {
    System.assertEquals(3, 1 + 1);
  }
}
