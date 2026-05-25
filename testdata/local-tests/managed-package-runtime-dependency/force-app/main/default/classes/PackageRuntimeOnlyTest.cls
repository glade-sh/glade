@IsTest
private class PackageRuntimeOnlyTest {
  @IsTest static void dependencyTestsDoNotRunForConsumer() {
    System.assertEquals('dependency-only', 'dependency-only');
  }
}
