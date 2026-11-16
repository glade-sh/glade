@IsTest private class PgProbe65T {
  @IsTest static void t() {
    GlGrant.run();
    System.assert(false, 'RESULT ' + PgProbe65.run());
  }
}
