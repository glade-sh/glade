@IsTest
private class ManageUsersCapture {
    static User makeUser(String alias) {
        Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User'];
        User u = new User(ProfileId = p.Id, Alias = alias, LastName = 'L2' + alias,
            Username = 'l2' + alias + Crypto.getRandomInteger() + '@example.invalid',
            Email = 'l2@example.invalid', TimeZoneSidKey = 'GMT', LocaleSidKey = 'en_US',
            EmailEncodingKey = 'UTF-8', LanguageLocaleKey = 'en_US');
        insert u;
        return u;
    }
    // Inserts a permission set with the given flags true and assigns it.
    static String grant(User u, List<String> flags) {
        PermissionSet ps = new PermissionSet(Name = 'L2_' + Crypto.getRandomInteger().toString().replace('-', 'm'), Label = 'L2 grant');
        for (String f : flags) { ps.put(f, true); }
        try {
            insert ps;
            insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = ps.Id);
        } catch (Exception e) {
            return e.getTypeName() + ': ' + e.getMessage();
        }
        return 'ok';
    }
    static Map<String,Object> attempt(SObject rec, Boolean allOrNone) {
        Map<String,Object> o = new Map<String,Object>();
        try {
            if (allOrNone) {
                update rec;
                o.put('outcome', 'ok');
            } else {
                Database.SaveResult r = Database.update(rec, false);
                o.put('outcome', r.isSuccess() ? 'ok' : 'saveResultError');
                if (!r.isSuccess()) {
                    o.put('statusCode', String.valueOf(r.getErrors()[0].getStatusCode()));
                    o.put('dmlMessage', r.getErrors()[0].getMessage());
                }
            }
        } catch (DmlException e) {
            o.put('outcome', 'DmlException');
            o.put('message', e.getMessage());
            o.put('statusCode', String.valueOf(e.getDmlType(0)));
            o.put('dmlMessage', e.getDmlMessage(0));
        } catch (Exception e) {
            o.put('outcome', e.getTypeName());
            o.put('message', e.getMessage());
        }
        return o;
    }
    static void emit(String probe, Map<String,Object> o) {
        System.assert(false, 'CAP|' + probe + '|' + JSON.serialize(o));
    }
    static String storedTitle(Id userId) {
        return [SELECT Title FROM User WHERE Id = :userId].Title;
    }
    static Map<String,Object> userPerms() {
        Map<String,Object> m = new Map<String,Object>();
        List<String> names = new List<String>{'PermissionsManageUsers', 'PermissionsManageInternalUsers', 'PermissionsModifyAllData', 'PermissionsViewAllUsers'};
        // UserInfo-style check through the effective permission set access of the running user.
        for (PermissionSetAssignment a : [SELECT PermissionSet.PermissionsManageUsers, PermissionSet.PermissionsManageInternalUsers, PermissionSet.PermissionsModifyAllData FROM PermissionSetAssignment WHERE AssigneeId = :UserInfo.getUserId()]) {
            if (a.PermissionSet.PermissionsManageUsers) m.put('ManageUsers', true);
            if (a.PermissionSet.PermissionsManageInternalUsers) m.put('ManageInternalUsers', true);
            if (a.PermissionSet.PermissionsModifyAllData) m.put('ModifyAllData', true);
        }
        return m;
    }
    // target: 'other' (another Standard User) or 'admin' (the running administrator).
    static void run(String probe, List<String> flags, String target, Boolean allOrNone) {
        User actor = makeUser('a' + probe.left(5));
        User other = makeUser('o' + probe.left(5));
        Id adminId = UserInfo.getUserId();
        Map<String,Object> o = new Map<String,Object>();
        if (flags != null) { o.put('grant', grant(actor, flags)); }
        Id targetId = target == 'admin' ? adminId : other.Id;
        System.runAs(actor) {
            o.put('perms', userPerms());
            o.put('isUpdateable', Schema.sObjectType.User.isUpdateable());
            o.put('titleUpdateable', Schema.sObjectType.User.fields.Title.isUpdateable());
            o.putAll(attempt(new User(Id = targetId, Title = 'L2 changed'), allOrNone));
        }
        o.put('storedTitle', storedTitle(targetId));
        emit(probe, o);
    }
    static List<String> manageUsers() {
        return new List<String>{'PermissionsManageUsers', 'PermissionsManageInternalUsers', 'PermissionsViewAllUsers', 'PermissionsViewSetup',
            'PermissionsManageRoles', 'PermissionsManageSharing', 'PermissionsResetPasswords', 'PermissionsManagePasswordPolicies',
            'PermissionsManageProfilesPermissionsets', 'PermissionsAssignPermissionSets', 'PermissionsManageIpAddresses',
            'PermissionsManageLoginAccessPolicies', 'PermissionsViewRoles', 'PermissionsApiEnabled',
            'PermissionsDelegatedTwoFactor', 'PermissionsFreezeUsers', 'PermissionsMonitorLoginHistory'};
    }
    static List<String> manageInternalUsers() {
        return new List<String>{'PermissionsManageInternalUsers', 'PermissionsViewAllUsers', 'PermissionsViewSetup', 'PermissionsResetPasswords', 'PermissionsViewRoles'};
    }
    @IsTest static void controlOther() { run('control.other', null, 'other', true); }
    @IsTest static void manageUsersOther() { run('manageUsers.other', manageUsers(), 'other', true); }
    @IsTest static void manageUsersOtherPartial() { run('manageUsers.otherPartial', manageUsers(), 'other', false); }
    @IsTest static void manageUsersAdmin() { run('manageUsers.admin', manageUsers(), 'admin', true); }
    @IsTest static void manageInternalUsersOther() { run('manageInternalUsers.other', manageInternalUsers(), 'other', true); }
    @IsTest static void manageInternalUsersOtherPartial() { run('manageInternalUsers.otherPartial', manageInternalUsers(), 'other', false); }
    @IsTest static void viewAllUsersOther() { run('viewAllUsers.other', new List<String>{'PermissionsViewAllUsers', 'PermissionsViewSetup', 'PermissionsViewRoles'}, 'other', true); }
    @IsTest static void manageInternalUsersAdmin() { run('manageInternalUsers.admin', manageInternalUsers(), 'admin', true); }
}
