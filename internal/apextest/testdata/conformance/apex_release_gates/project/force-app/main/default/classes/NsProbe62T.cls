@IsTest private class NsProbe62T {
  @IsTest static void t() {
    GlGrant.run();
    System.assert(false, 'RESULT ' + NsProbe62.run());
  }
}
