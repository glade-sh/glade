@isTest
private class UnsupportedTest {
  @isTest static void unsupported() {
    Pattern.compile('a', Pattern.COMMENTS);
  }
}
