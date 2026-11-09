@IsTest
private class KnowInternalTest {
  static String top(String c) { try { ConnectApi.Knowledge.getTopViewedArticlesForTopic(c, '0TO000000000001', 1); return 'OK'; } catch (Exception e) { return e.getTypeName() + '|' + e.getMessage(); } }
  @IsTest static void observe() {
    List<String> rows = new List<String>();
    for (String c : new List<String>{'internal', 'Internal', 'INTERNAL', ' internal', 'internal ', 'default', 'external'}) {
      rows.add('top\t[' + c + ']\t' + top(c));
    }
    System.assert(false, 'ROWS\n' + String.join(rows, '\n') + '\nEND');
  }
}
