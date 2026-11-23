@IsTest
private class R5InvProbe {
  @IsTest static void sizes() {
    System.assert(false, 'R5|inv|' + R5Inv.run());
  }
}
