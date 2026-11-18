@IsTest private class IaProbe64T {
  @IsTest static void t() {
    GlGrant.run();
    System.assert(false, 'RESULT ' + IaProbe64.run());
  }
}
