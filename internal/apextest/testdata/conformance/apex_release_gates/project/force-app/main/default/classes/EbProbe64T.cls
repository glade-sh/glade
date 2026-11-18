@IsTest private class EbProbe64T {
  @IsTest static void t() {
    GlGrant.run();
    System.assert(false, 'RESULT ' + EbProbe64.run());
  }
}
