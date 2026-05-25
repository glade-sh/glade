@IsTest
private class TrailAccessTest {
  @IsTest static void callsDependencyGlobalApi() {
    System.assertEquals('from dependency', pkgx.TrailService.ok());
  }
}
