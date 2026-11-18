@IsTest private class PgProbe67T {
  @IsTest static void t() {
    GlGrant.run();
    System.assert(false, 'RESULT ' + PgProbe67.run());
  }
}
