@IsTest private class RpModeProbe62T {
  @IsTest static void t() {
    GlGrant.run();
    System.assert(false, 'RESULT ' + RpModeProbe62.run());
  }
}
