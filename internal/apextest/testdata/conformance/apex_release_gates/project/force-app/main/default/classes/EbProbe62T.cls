@IsTest private class EbProbe62T {
  @IsTest static void t() {
    GlGrant.run();
    System.assert(false, 'RESULT ' + EbProbe62.run());
  }
}
