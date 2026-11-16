@IsTest private class PgProbe62T {
  @IsTest static void t() {
    GlGrant.run();
    System.assert(false, 'RESULT ' + PgProbe62.run());
  }
}
