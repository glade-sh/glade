@IsTest private class NsProbe67T {
  @IsTest static void t() {
    GlGrant.run();
    System.assert(false, 'RESULT ' + NsProbe67.run());
  }
}
