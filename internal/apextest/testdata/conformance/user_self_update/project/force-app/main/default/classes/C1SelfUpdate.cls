@IsTest
private class C1SelfUpdate {
    static User makeUser(String alias) {
        Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User'];
        User u = new User(ProfileId = p.Id, Alias = alias, LastName = 'Self' + alias,
            Username = 'c1' + alias + Crypto.getRandomInteger() + '@example.invalid',
            Email = 'c1@example.invalid', TimeZoneSidKey = 'GMT', LocaleSidKey = 'en_US',
            EmailEncodingKey = 'UTF-8', LanguageLocaleKey = 'en_US');
        insert u;
        return u;
    }
    static Map<String,Object> attempt(SObject rec) {
        Map<String,Object> o = new Map<String,Object>();
        try {
            update rec;
            o.put('outcome', 'ok');
        } catch (DmlException e) {
            o.put('outcome', 'DmlException');
            o.put('message', e.getMessage());
            o.put('numDml', e.getNumDml());
            o.put('statusCode', String.valueOf(e.getDmlType(0)));
            o.put('dmlMessage', e.getDmlMessage(0));
            List<String> names = new List<String>();
            List<Schema.SObjectField> fields = e.getDmlFields(0);
            if (fields == null) { names = null; } else { for (Schema.SObjectField f : fields) { names.add(String.valueOf(f)); } }
            o.put('dmlFields', names);
            o.put('dmlFieldNames', e.getDmlFieldNames(0));
        } catch (Exception e) {
            o.put('outcome', e.getTypeName());
            o.put('message', e.getMessage());
        }
        return o;
    }
    static Map<String,Object> stored(Id userId) {
        User s = [SELECT LocaleSidKey, TimeZoneSidKey, Email, Phone, FirstName, Title, Alias, LastName, LanguageLocaleKey, EmailEncodingKey FROM User WHERE Id = :userId];
        return new Map<String,Object>{'LocaleSidKey'=>s.LocaleSidKey,'TimeZoneSidKey'=>s.TimeZoneSidKey,'Email'=>s.Email,'Phone'=>s.Phone,'FirstName'=>s.FirstName,'Title'=>s.Title,'Alias'=>s.Alias,'LastName'=>s.LastName,'LanguageLocaleKey'=>s.LanguageLocaleKey,'EmailEncodingKey'=>s.EmailEncodingKey};
    }
    static void emit(String probe, Map<String,Object> o) {
        System.assert(false, 'CAP|' + probe + '|' + JSON.serialize(o));
    }
    static SObject byId(Id userId, String field) {
        User r = new User(Id = userId);
        if (field == 'LocaleSidKey') r.LocaleSidKey = 'tr_TR';
        else if (field == 'TimeZoneSidKey') r.TimeZoneSidKey = 'America/New_York';
        else if (field == 'Email') r.Email = 'c1changed@example.invalid';
        else if (field == 'Phone') r.Phone = '555-0100';
        else if (field == 'FirstName') r.FirstName = 'Changed';
        else if (field == 'Title') r.Title = 'Changed Title';
        else if (field == 'Alias') r.Alias = 'chg';
        else if (field == 'LastName') r.LastName = 'ChangedLast';
        else if (field == 'LanguageLocaleKey') r.LanguageLocaleKey = 'en_US';
        else if (field == 'EmailEncodingKey') r.EmailEncodingKey = 'UTF-8';
        else if (field == 'ProfileId') r.ProfileId = [SELECT ProfileId FROM User WHERE Id = :userId].ProfileId;
        else if (field == 'Username') r.Username = [SELECT Username FROM User WHERE Id = :userId].Username;
        return r;
    }
    static void runById(String field) {
        User u = makeUser('f' + field.left(6));
        Map<String,Object> o;
        System.runAs(u) { o = attempt(byId(u.Id, field)); }
        o.put('stored', stored(u.Id));
        emit('byid.' + field, o);
    }
    @IsTest static void byIdLocaleSidKey() { runById('LocaleSidKey'); }
    @IsTest static void byIdTimeZoneSidKey() { runById('TimeZoneSidKey'); }
    @IsTest static void byIdEmail() { runById('Email'); }
    @IsTest static void byIdPhone() { runById('Phone'); }
    @IsTest static void byIdFirstName() { runById('FirstName'); }
    @IsTest static void byIdTitle() { runById('Title'); }
    @IsTest static void byIdAlias() { runById('Alias'); }
    @IsTest static void byIdLastName() { runById('LastName'); }
    @IsTest static void byIdLanguageLocaleKey() { runById('LanguageLocaleKey'); }
    @IsTest static void byIdEmailEncodingKey() { runById('EmailEncodingKey'); }
    @IsTest static void byIdProfileId() { runById('ProfileId'); }
    @IsTest static void byIdUsername() { runById('Username'); }

    // The record queried inside runAs with the six fields, all changed, then updated.
    @IsTest static void queriedSix() {
        User u = makeUser('q6');
        Map<String,Object> o;
        System.runAs(u) {
            User q = [SELECT Id, LocaleSidKey, TimeZoneSidKey, Email, Phone, FirstName, Title FROM User WHERE Id = :u.Id];
            q.LocaleSidKey = 'tr_TR'; q.TimeZoneSidKey = 'America/New_York'; q.Email = 'c1changed@example.invalid';
            q.Phone = '555-0100'; q.FirstName = 'Changed'; q.Title = 'Changed Title';
            o = attempt(q);
        }
        o.put('stored', stored(u.Id));
        emit('queried.six', o);
    }
    // The record queried inside runAs with the six fields, unchanged except LocaleSidKey.
    @IsTest static void queriedSixLocaleOnly() {
        User u = makeUser('q6l');
        Map<String,Object> o;
        System.runAs(u) {
            User q = [SELECT Id, LocaleSidKey, TimeZoneSidKey, Email, Phone, FirstName, Title FROM User WHERE Id = :u.Id];
            q.LocaleSidKey = 'tr_TR';
            o = attempt(q);
        }
        o.put('stored', stored(u.Id));
        emit('queried.sixLocaleOnly', o);
    }
    // The record queried with every field the insert set, LocaleSidKey changed.
    @IsTest static void queriedInsertFields() {
        User u = makeUser('qi');
        Map<String,Object> o;
        System.runAs(u) {
            User q = [SELECT Id, ProfileId, Alias, LastName, Username, Email, TimeZoneSidKey, LocaleSidKey, EmailEncodingKey, LanguageLocaleKey FROM User WHERE Id = :u.Id];
            q.LocaleSidKey = 'tr_TR';
            o = attempt(q);
        }
        o.put('stored', stored(u.Id));
        emit('queried.insertFields', o);
    }
    // Admin queries the record (insert fields), runAs updates that object.
    @IsTest static void adminQueriedInsertFields() {
        User u = makeUser('aq');
        User q = [SELECT Id, ProfileId, Alias, LastName, Username, Email, TimeZoneSidKey, LocaleSidKey, EmailEncodingKey, LanguageLocaleKey FROM User WHERE Id = :u.Id];
        Map<String,Object> o;
        System.runAs(u) { q.LocaleSidKey = 'tr_TR'; o = attempt(q); }
        o.put('stored', stored(u.Id));
        emit('adminQueried.insertFields', o);
    }
    // The inserted User object itself (lane I p3/p4 shape).
    @IsTest static void insertedObject() {
        User u = makeUser('io');
        Map<String,Object> o;
        System.runAs(u) { u.LocaleSidKey = 'tr_TR'; o = attempt(u); }
        o.put('stored', stored(u.Id));
        emit('inserted.object', o);
    }
    // ProfileId and Username only (same values), queried inside runAs.
    @IsTest static void queriedProfileUsername() {
        User u = makeUser('qp');
        Map<String,Object> o;
        System.runAs(u) {
            User q = [SELECT Id, ProfileId, Username FROM User WHERE Id = :u.Id];
            o = attempt(q);
        }
        o.put('stored', stored(u.Id));
        emit('queried.profileUsername', o);
    }
    // Another user's record (the admin) by Id, Title.
    @IsTest static void otherUserTitle() {
        User u = makeUser('ot');
        Id adminId = UserInfo.getUserId();
        Map<String,Object> o;
        System.runAs(u) { o = attempt(new User(Id = adminId, Title = 'Changed Title')); }
        emit('other.title', o);
    }
    // Another Standard User's record by Id, Title.
    @IsTest static void otherStandardUserTitle() {
        User u = makeUser('ou');
        User v = makeUser('ov');
        Map<String,Object> o;
        System.runAs(u) { o = attempt(new User(Id = v.Id, Title = 'Changed Title')); }
        emit('otherStandard.title', o);
    }

    // Describe of User fields as the Standard User.
    @IsTest static void describeAsStandard() {
        User u = makeUser('ds');
        Map<String,Object> o = new Map<String,Object>();
        System.runAs(u) {
            Map<String, Schema.SObjectField> m = Schema.SObjectType.User.fields.getMap();
            o.put('objectUpdateable', Schema.SObjectType.User.isUpdateable());
            List<String> names = new List<String>{'ProfileId','UserRoleId','IsActive','Username','Alias','LastName','FirstName','Email','Phone','Title','LocaleSidKey','TimeZoneSidKey','LanguageLocaleKey','EmailEncodingKey','CommunityNickname','ManagerId','Department','UserPermissionsMarketingUser'};
            for (String n : names) {
                Schema.DescribeFieldResult d = m.get(n).getDescribe();
                o.put(n, new Map<String,Object>{'updateable'=>d.isUpdateable(),'accessible'=>d.isAccessible(),'createable'=>d.isCreateable(),'permissionable'=>d.isPermissionable()});
            }
        }
        emit('describe.standard', o);
    }
    @IsTest static void byIdUserRoleId() { User u = makeUser('rr'); Map<String,Object> o; System.runAs(u) { o = attempt(new User(Id = u.Id, UserRoleId = null)); } emit('byid.UserRoleId', o); }
    @IsTest static void byIdIsActive() { User u = makeUser('ia'); Map<String,Object> o; System.runAs(u) { o = attempt(new User(Id = u.Id, IsActive = true)); } emit('byid.IsActive', o); }
    @IsTest static void byIdManagerId() { User u = makeUser('mg'); Map<String,Object> o; System.runAs(u) { o = attempt(new User(Id = u.Id, ManagerId = null)); } emit('byid.ManagerId', o); }
    @IsTest static void byIdDepartment() { User u = makeUser('dp'); Map<String,Object> o; System.runAs(u) { o = attempt(new User(Id = u.Id, Department = 'Dept')); } emit('byid.Department', o); }
    @IsTest static void byIdCommunityNickname() { User u = makeUser('cn'); Map<String,Object> o; System.runAs(u) { o = attempt(new User(Id = u.Id, CommunityNickname = 'c1nick' + Crypto.getRandomInteger())); } emit('byid.CommunityNickname', o); }
    @IsTest static void queriedRoleActive() {
        User u = makeUser('qr');
        Map<String,Object> o;
        System.runAs(u) { User q = [SELECT Id, UserRoleId, IsActive, ManagerId FROM User WHERE Id = :u.Id]; o = attempt(q); }
        emit('queried.roleActiveManager', o);
    }
    // ProfileId assigned on a fresh User instance outside runAs, updated inside runAs.
    @IsTest static void profileIdSetOutsideRunAs() {
        User u = makeUser('po');
        User r = new User(Id = u.Id, ProfileId = u.ProfileId);
        Map<String,Object> o;
        System.runAs(u) { o = attempt(r); }
        emit('byid.ProfileIdOutside', o);
    }

    // ManagerId and ProfileId both set outside runAs; allOrNone statement.
    @IsTest static void twoInaccessible() {
        User u = makeUser('tw');
        User r = new User(Id = u.Id, ManagerId = null, ProfileId = u.ProfileId, Title = 'T');
        Map<String,Object> o;
        System.runAs(u) { o = attempt(r); }
        o.put('stored', stored(u.Id));
        emit('byid.twoInaccessible', o);
    }
    // Database.update(rec, false) with ProfileId set.
    @IsTest static void partialProfileId() {
        User u = makeUser('pp');
        User r = new User(Id = u.Id, ProfileId = u.ProfileId, Title = 'T');
        Map<String,Object> o = new Map<String,Object>();
        System.runAs(u) {
            Database.SaveResult sr = Database.update(r, false);
            o.put('success', sr.isSuccess());
            List<Object> errs = new List<Object>();
            for (Database.Error e : sr.getErrors()) { errs.add(new Map<String,Object>{'statusCode'=>String.valueOf(e.getStatusCode()),'message'=>e.getMessage(),'fields'=>e.getFields()}); }
            o.put('errors', errs);
        }
        o.put('stored', stored(u.Id));
        emit('partial.ProfileId', o);
    }
    // Field assignment statement inside runAs on a non-updateable field.
    @IsTest static void assignManagerInside() {
        User u = makeUser('am');
        Map<String,Object> o = new Map<String,Object>();
        System.runAs(u) {
            User r = new User(Id = u.Id);
            try { r.ManagerId = null; o.put('assign', 'ok'); } catch (Exception e) { o.put('assign', e.getTypeName() + ': ' + e.getMessage()); }
        }
        emit('assign.ManagerIdInside', o);
    }
}
