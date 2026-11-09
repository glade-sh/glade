@isTest class StripDefaultProbe55 {
  static User user(String profile) {
    Profile p = [SELECT Id FROM Profile WHERE Name = :profile];
    return new User(Alias='sdp', Email='strip@example.test', EmailEncodingKey='UTF-8', LastName='Strip', LanguageLocaleKey='en_US', LocaleSidKey='en_US', ProfileId=p.Id, TimeZoneSidKey='America/Los_Angeles', UserName='strip' + System.now().getTime() + '55' + profile.length() + '@example.test');
  }
  static List<SObject> fresh() {
    return new List<SObject>{
      new Account(Name='Test'), new Contact(LastName='Test'), new Lead(LastName='Test', Company='Test'),
      new Case(Subject='Test'), new Opportunity(Name='Test', StageName='Prospecting', CloseDate=Date.newInstance(2030,1,1)),
      new Task(Subject='Test'), new Product2(Name='Test'), new Pricebook2(Name='Test'), new Campaign(Name='Test'),
      new StripProbe__c(Name='Test')
    };
  }
  static List<SObject> explicitFalse() {
    return new List<SObject>{
      new Contact(LastName='Test', DoNotCall=false), new Lead(LastName='Test', Company='Test', DoNotCall=false),
      new Case(Subject='Test', IsEscalated=false), new Product2(Name='Test', IsActive=false),
      new Pricebook2(Name='Test', IsActive=false), new Campaign(Name='Test', IsActive=false),
      new StripProbe__c(Name='Test', Flag__c=false)
    };
  }
  static List<SObject> loadDefaults() {
    List<SObject> rows = new List<SObject>();
    for (SObject row : fresh()) rows.add(row.getSObjectType().newSObject(null, true));
    return rows;
  }
  static String sorted(Set<String> values) {
    List<String> items = new List<String>(values);
    items.sort();
    return String.join(items, ',');
  }
  static String probe(AccessType access, SObject row, Boolean doInsert) {
    String name = String.valueOf(row.getSObjectType());
    String out = name + ' populated=' + sorted(row.getPopulatedFieldsAsMap().keySet());
    try {
      SObjectAccessDecision d = Security.stripInaccessible(access, new List<SObject>{row}, true);
      Set<String> removed = d.getRemovedFields().get(name);
      out += ' removed=' + (removed == null ? '' : sorted(removed));
      if (doInsert) {
        try { insert d.getRecords(); out += ' insert=ok'; }
        catch (DmlException e) { out += ' insert=' + e.getDmlType(0); }
      }
    } catch (Exception e) {
      out += ' threw=' + e.getTypeName();
    }
    return out + '\n';
  }
  static void report(String profile, AccessType access, List<SObject> rows, Boolean doInsert) {
    String out = '';
    System.runAs(user(profile)) {
      for (SObject row : rows) out += probe(access, row, doInsert);
    }
    System.assert(false, '\n' + out);
  }
  static List<SObject> saved() {
    List<SObject> rows = fresh();
    insert rows;
    return rows;
  }
  static List<SObject> byId() {
    List<SObject> rows = new List<SObject>();
    for (SObject row : saved()) rows.add(row.getSObjectType().newSObject(row.Id));
    return rows;
  }
  static List<SObject> queried() {
    List<SObject> rows = new List<SObject>();
    for (SObject row : saved()) rows.add(Database.query('SELECT Id FROM ' + row.getSObjectType() + ' WHERE Id = \'' + row.Id + '\''));
    return rows;
  }
  static final String A = 'System Administrator', S = 'Standard User', M = 'Minimum Access - Salesforce';
  @isTest static void freshCreatableAdmin() { report(A, AccessType.CREATABLE, fresh(), true); }
  @isTest static void freshCreatableStandard() { report(S, AccessType.CREATABLE, fresh(), true); }
  @isTest static void freshCreatableMinimal() { report(M, AccessType.CREATABLE, fresh(), false); }
  @isTest static void freshUpdatableStandard() { report(S, AccessType.UPDATABLE, fresh(), false); }
  @isTest static void explicitCreatableAdmin() { report(A, AccessType.CREATABLE, explicitFalse(), false); }
  @isTest static void explicitCreatableStandard() { report(S, AccessType.CREATABLE, explicitFalse(), false); }
  @isTest static void loadDefaultsCreatableAdmin() { report(A, AccessType.CREATABLE, loadDefaults(), false); }
  @isTest static void loadDefaultsCreatableStandard() { report(S, AccessType.CREATABLE, loadDefaults(), false); }
  @isTest static void byIdUpdatableStandard() { report(S, AccessType.UPDATABLE, byId(), false); }
  @isTest static void queriedUpdatableStandard() { report(S, AccessType.UPDATABLE, queried(), false); }
}
