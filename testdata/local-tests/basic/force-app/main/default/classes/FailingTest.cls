@isTest
private class FailingTest {
  @isTest static void fails() {
    System.assertEquals(2, 1 + 1);
  }
}
