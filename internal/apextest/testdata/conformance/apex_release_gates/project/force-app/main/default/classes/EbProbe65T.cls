@IsTest private class EbProbe65T {
  @IsTest static void t() {
    GlGrant.run();
    System.assert(false, 'RESULT ' + EbProbe65.run());
  }
}
