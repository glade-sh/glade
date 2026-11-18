@IsTest
private class Dt9HalfTest {
 @IsTest static void observe() {
  List<String> rows = new List<String>();
  Datetime a = Datetime.newInstanceGmt(2026, 1, 8, 10, 30, 0);
  Datetime b = Datetime.newInstanceGmt(2026, 7, 8, 10, 30, 0);
  List<User> us = new List<User>();
  List<String> zones = new List<String>{'Asia/Kolkata', 'Australia/Adelaide', 'Asia/Kathmandu'};
  List<String> locs = new List<String>{'en_GB', 'de_DE', 'ja_JP'};
  List<String> langs = new List<String>{'en_US', 'de', 'ja'};
  Integer n = 0;
  for (Integer z = 0; z < zones.size(); z++) {
   for (Integer l = 0; l < locs.size(); l++) {
    n++;
    us.add(DtSupport.mk(locs[l], zones[z], 'h' + n, langs[l]));
   }
  }
  for (User u : us) {
   System.runAs(u) {
    rows.add(DtSupport.tag(u) + '=' + a.formatLong() + '|' + b.formatLong() + '|z=' + a.format('z Z') + '|' + b.format('z Z'));
   }
  }
  System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
