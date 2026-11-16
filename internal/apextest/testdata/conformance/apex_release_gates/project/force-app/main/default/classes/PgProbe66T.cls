@IsTest private class PgProbe66T {
  @IsTest static void t() {
    GlGrant.run();
    System.assert(false, 'RESULT ' + PgProbe66.run());
  }
}
