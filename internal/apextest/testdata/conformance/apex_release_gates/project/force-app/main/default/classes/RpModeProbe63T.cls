@IsTest private class RpModeProbe63T {
  @IsTest static void t() {
    GlGrant.run();
    System.assert(false, 'RESULT ' + RpModeProbe63.run());
  }
}
