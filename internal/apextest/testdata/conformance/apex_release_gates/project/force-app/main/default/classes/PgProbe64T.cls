@IsTest private class PgProbe64T {
  @IsTest static void t() {
    GlGrant.run();
    System.assert(false, 'RESULT ' + PgProbe64.run());
  }
}
