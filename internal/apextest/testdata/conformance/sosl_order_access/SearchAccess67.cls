@IsTest private class SearchAccess67 {
  static User mkUser() {
    Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
    String un = 'gl' + String.valueOf(Crypto.getRandomInteger()).replace('-','') + '@example.com';
    User u = new User(Alias='gl', Email=un, EmailEncodingKey='UTF-8', LastName='GlStd', LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='America/Los_Angeles', Username=un, ProfileId=p.Id);
    PermissionSet ps = [SELECT Id FROM PermissionSet WHERE Name = 'Gl_Access'];
    PermissionSet pa = [SELECT Id FROM PermissionSet WHERE Name = 'Gl_Admin_Access'];
    System.runAs(new User(Id = UserInfo.getUserId())) {
      insert u;
      insert new PermissionSetAssignment(AssigneeId=u.Id, PermissionSetId=ps.Id);
      insert new PermissionSetAssignment(AssigneeId=UserInfo.getUserId(), PermissionSetId=pa.Id);
    }
    return u;
  }

    static void observe(String selection, String candidates, Boolean useFind) {
        User restricted = mkUser();
        Gl_Obj__c mine = new Gl_Obj__c(Name='Mine', Open__c='Visible', Secret__c='Constant', OwnerId=restricted.Id);
        Gl_Obj__c other = new Gl_Obj__c(Name='Other', Open__c='Hidden', Secret__c='Constant');
        insert new List<Gl_Obj__c>{other, mine};
        String answer;
        System.runAs(restricted) {
            List<Id> ids = new List<Id>();
            if (candidates == 'mixed') ids.add(mine.Id);
            if (candidates != 'empty') ids.add(other.Id);
            Test.setFixedSearchResults(ids);
            try {
                String query = 'FIND {unrelated} RETURNING Gl_Obj__c(' + selection + ') WITH USER_MODE';
                List<String> labels = new List<String>();
                if (useFind) {
                    for (Search.SearchResult hit : Search.find(query).get('Gl_Obj__c')) {
                        SObject row = hit.getSObject();
                        labels.add((Id)row.get('Id') == mine.Id ? 'mine' : 'other');
                    }
                } else {
                    for (SObject row : Search.query(query)[0]) {
                        labels.add((Id)row.get('Id') == mine.Id ? 'mine' : 'other');
                    }
                }
                answer = 'records=' + String.join(labels, ',');
            } catch(Exception e) {
                answer = e.getTypeName() + '|' + e.getMessage();
            }
        }
        System.assert(false, 'OBS|' + answer);
    }
    @IsTest static void orderReadable() { observe('Id ORDER BY Open__c', 'mixed', false); }
    @IsTest static void selectUnreadable() { observe('Id, Secret__c', 'mixed', false); }
    @IsTest static void filterUnreadable() { observe('Id WHERE Secret__c != null', 'mixed', false); }
    @IsTest static void orderUnreadable() { observe('Id ORDER BY Secret__c', 'mixed', false); }
    @IsTest static void secondOrderUnreadable() { observe('Id ORDER BY Name, Secret__c', 'mixed', false); }
    @IsTest static void onlyOtherOwner() { observe('Id ORDER BY Open__c', 'other', false); }
    @IsTest static void onlyOtherOwnerOrderUnreadable() { observe('Id ORDER BY Secret__c', 'other', false); }
    @IsTest static void noCandidatesOrderUnreadable() { observe('Id ORDER BY Secret__c', 'empty', false); }
    @IsTest static void findOrderUnreadable() { observe('Id ORDER BY Secret__c', 'mixed', true); }
    @IsTest static void findOrderReadable() { observe('Id ORDER BY Open__c', 'mixed', true); }
}
