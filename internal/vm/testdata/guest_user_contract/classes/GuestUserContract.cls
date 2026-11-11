@IsTest private class GuestUserContract {
  static String b(Boolean v) { return v ? '1' : '0'; }
  static User mk(String profileName, String suffix) {
    Profile p = [SELECT Id FROM Profile WHERE Name = :profileName LIMIT 1];
    return new User(Alias='gst',Email='o@example.invalid',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LastName='Doe',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',Username='gu.'+UserInfo.getOrganizationId()+'.'+Datetime.now().getTime()+suffix+'@example.invalid');
  }
  static User guest() {
    List<Profile> ps = [SELECT Name FROM Profile WHERE UserType = 'Guest' ORDER BY Name];
    System.assert(!ps.isEmpty(), 'no guest profile');
    return mk(ps[0].Name, 'g');
  }
  static Map<String,String> flags() {
    Map<String,String> m = new Map<String,String>();
    Map<String,Schema.SObjectField> fm = Schema.SObjectType.User.fields.getMap();
    for (String n : fm.keySet()) {
      Schema.DescribeFieldResult d = fm.get(n).getDescribe();
      m.put(n, b(d.isAccessible()) + b(d.isCreateable()) + b(d.isUpdateable()));
    }
    return m;
  }
  @IsTest static void describeFlags() {
    Schema.DescribeSObjectResult o = Schema.SObjectType.User;
    String obj = b(o.isAccessible()) + b(o.isCreateable()) + b(o.isUpdateable()) + b(o.isDeletable()) + b(o.isQueryable());
    Map<String,String> g, s;
    System.runAs(guest()) { obj += '/' + b(Schema.SObjectType.User.isAccessible()) + b(Schema.SObjectType.User.isCreateable()) + b(Schema.SObjectType.User.isUpdateable()) + b(Schema.SObjectType.User.isDeletable()) + b(Schema.SObjectType.User.isQueryable()); g = flags(); }
    System.runAs(mk('Standard User','s')) { s = flags(); }
    List<String> names = new List<String>(g.keySet()); names.sort();
    List<String> diffs = new List<String>();
    for (String n : names) if (n != 'numberoffailedlogins' && g.get(n) != s.get(n)) diffs.add(n + ':' + g.get(n));
    System.assert(false, 'obj=' + obj + ' diffs=' + String.join(diffs, ','));
  }
  @IsTest static void soqlRows() {
    User ins = guest(); insert ins;
    String r = '';
    System.runAs(ins) {
      String me = UserInfo.getUserId();
      for (String n : 'accountid,callcenterid,contactid,individualid,profileid,userroleid,numberoffailedlogins,userpreferencesapexpagesdevelopermode,userpreferencesreverseopenactivitiesview,name,lastname,email,alias,isactive,username,managerid,timezonesidkey,usertype'.split(',')) {
        try { Database.query('SELECT ' + n + ' FROM User WHERE Id = \'' + me + '\''); r += n + '=ok '; }
        catch (Exception e) { r += n + '=' + e.getTypeName() + ' '; }
      }
      List<User> all = [SELECT Id FROM User];
      r += '|rows=' + all.size() + ' self=' + (all.size() > 0 && all[0].Id == ins.Id);
      r += ' |where=' + [SELECT Id FROM User WHERE IsActive = true].size();
    }
    System.assert(false, r);
  }
}
