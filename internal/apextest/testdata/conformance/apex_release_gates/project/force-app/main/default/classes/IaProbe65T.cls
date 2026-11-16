@IsTest private class IaProbe65T {
  @IsTest static void t() {
    GlGrant.run();
    System.assert(false, 'RESULT ' + IaProbe65.run());
  }
}
