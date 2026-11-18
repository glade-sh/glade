@IsTest private class PgProbe63T {
  @IsTest static void t() {
    GlGrant.run();
    System.assert(false, 'RESULT ' + PgProbe63.run());
  }
}
