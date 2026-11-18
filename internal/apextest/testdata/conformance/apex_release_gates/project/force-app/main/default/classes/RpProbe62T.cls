@IsTest private class RpProbe62T {
  @IsTest static void t() {
    GlGrant.run();
    System.assert(false, 'RESULT ' + RpProbe62.run());
  }
}
