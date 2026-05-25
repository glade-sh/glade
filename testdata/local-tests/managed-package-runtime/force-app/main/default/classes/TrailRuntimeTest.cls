@IsTest
private class TrailRuntimeTest {
  @IsTest static void callsDependencyGlobalApi() {
    System.assertEquals('from dependency', pkgx.TrailRuntimeService.externalValue());
  }
}
