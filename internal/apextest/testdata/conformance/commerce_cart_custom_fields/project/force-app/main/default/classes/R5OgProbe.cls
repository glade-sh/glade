@IsTest
private class R5OgProbe {
  @IsTest static void custom() { System.assert(false, 'R5|cf|' + R5Cf.run()); }
}
