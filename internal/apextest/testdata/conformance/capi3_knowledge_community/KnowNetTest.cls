@IsTest
private class KnowNetTest {
  static List<SObject> nets() { return Schema.getGlobalDescribe().containsKey('Network') ? Database.query('SELECT Id, Name FROM Network ORDER BY Name') : new List<SObject>(); }
  static String top(String c) { try { ConnectApi.Knowledge.getTopViewedArticlesForTopic(c, '0TO000000000001', 1); return 'OK'; } catch (Exception e) { return e.getTypeName() + '|' + e.getMessage(); } }
  static String show(String s, Id real) { return real == null ? s : s.replace(real, '{realNetwork}'); }
  static void fail(List<String> rows) { System.assert(false, 'ROWS\n' + String.join(rows, '\n') + '\nEND'); }
  @IsTest static void observeTop() {
    List<String> rows = new List<String>();
    List<SObject> nets = nets();
    rows.add('networkCount\t' + nets.size());
    Id real = nets.isEmpty() ? null : (Id)nets[0].get('Id');
    String real15 = real == null ? null : String.valueOf(real).left(15);
    Map<String,String> inputs = new Map<String,String>{
      'null' => null,
      'realId18' => real == null ? 'NONE' : String.valueOf(real),
      'realId15' => real15 == null ? 'NONE' : real15,
      'fake0DB18' => '0DB000000000001AAA',
      'fake0DB15' => '0DB000000000001',
      'fake0DBother' => '0DB0X0000000099AAA',
      'internal' => 'internal',
      'me' => 'me',
      'fake001' => '001000000000001AAA',
      'userId' => String.valueOf(UserInfo.getUserId()),
      'fake0DBmalformed' => '0DB00000000000'
    };
    for (String k : new List<String>{'null','realId18','realId15','fake0DB18','fake0DB15','fake0DBother','internal','me','fake001','userId','fake0DBmalformed'}) {
      if (inputs.get(k) == 'NONE') continue;
      rows.add('top\t' + k + '\t' + show(top(inputs.get(k)), real).replace(String.valueOf(UserInfo.getUserId()), '{userId}'));
    }
    fail(rows);
  }
  @IsTest static void observeTrendingFake() {
    String r;
    try { ConnectApi.Knowledge.getTrendingArticles('0DB000000000001', 1); r = 'OK'; } catch (Exception e) { r = e.getTypeName() + '|' + e.getMessage(); }
    String r2;
    try { ConnectApi.Knowledge.getTrendingArticlesForTopic('0DB000000000001', '0TO000000000001', 1); r2 = 'OK'; } catch (Exception e) { r2 = e.getTypeName() + '|' + e.getMessage(); }
    fail(new List<String>{'trending\tfake0DB15\t' + r, 'trendTopic\tfake0DB15\t' + r2});
  }
  @IsTest static void observeTrendingReal() {
    List<SObject> nets = nets();
    if (nets.isEmpty()) { fail(new List<String>{'trendingReal\tno-network'}); }
    String r;
    try { ConnectApi.Knowledge.getTrendingArticles((Id)nets[0].get('Id'), 1); r = 'OK'; } catch (Exception e) { r = e.getTypeName() + '|' + e.getMessage(); }
    fail(new List<String>{'trendingReal\treal\t' + r});
  }
  @IsTest static void observeTrendTopicReal() {
    List<SObject> nets = nets();
    if (nets.isEmpty()) { fail(new List<String>{'trendTopicReal\tno-network'}); }
    String r;
    try { ConnectApi.Knowledge.getTrendingArticlesForTopic((Id)nets[0].get('Id'), '0TO000000000001', 1); r = 'OK'; } catch (Exception e) { r = e.getTypeName() + '|' + e.getMessage(); }
    fail(new List<String>{'trendTopicReal\treal\t' + r});
  }
}
