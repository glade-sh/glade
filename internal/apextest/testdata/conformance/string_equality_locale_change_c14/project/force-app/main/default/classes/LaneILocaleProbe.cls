@IsTest
private class LaneILocaleProbe {
    static String upperI = String.fromCharArray(new List<Integer>{73});
    static String lowerI = String.fromCharArray(new List<Integer>{105});
    static String dotted = String.fromCharArray(new List<Integer>{304});
    static String fi = String.fromCharArray(new List<Integer>{64257});
    static String fiAscii = String.fromCharArray(new List<Integer>{102,105});

    static Map<String,Object> step() {
        Map<String,Object> m = new Map<String,Object>();
        m.put('I_i', upperI == lowerI);
        m.put('dotted_i', dotted == lowerI);
        m.put('fi_lig_fi', fi == fiAscii);
        m.put('I_i_eqic', upperI.equalsIgnoreCase(lowerI));
        m.put('i_upper', lowerI.toUpperCase().getChars());
        m.put('locale', UserInfo.getLocale());
        m.put('queried', [SELECT LocaleSidKey FROM User WHERE Id = :UserInfo.getUserId()].LocaleSidKey);
        return m;
    }
    static void emit(String id, Map<String,Object> o) {
        System.assert(false, 'CAP|' + id + '|' + JSON.serialize(o));
    }
    static User makeUser(String locale) {
        Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
        User u = new User(Alias = 'lnprobe', Email = 'laneiprobe@example.com', EmailEncodingKey = 'UTF-8',
            LastName = 'Probe', LanguageLocaleKey = 'en_US', LocaleSidKey = locale, ProfileId = p.Id,
            TimeZoneSidKey = 'GMT', UserName = 'laneiprobe.' + System.currentTimeMillis() + '@example.com');
        insert u;
        return u;
    }

    // Probe 2a: admin updates its own LocaleSidKey mid-test, no startTest.
    @IsTest static void p2aOwnUpdate() {
        Map<String,Object> o = new Map<String,Object>();
        o.put('before', step());
        update new User(Id = UserInfo.getUserId(), LocaleSidKey = 'tr_TR');
        o.put('after_update', step());
        emit('n6.p2a', o);
    }
    // Probe 2b: admin updates its own LocaleSidKey, then Test.startTest() before the second evaluation.
    @IsTest static void p2bOwnUpdateStartTest() {
        Map<String,Object> o = new Map<String,Object>();
        o.put('before', step());
        update new User(Id = UserInfo.getUserId(), LocaleSidKey = 'tr_TR');
        Test.startTest();
        o.put('after_start', step());
        Test.stopTest();
        o.put('after_stop', step());
        emit('n6.p2b', o);
    }
    // Probe 3: en_US user U, updated to tr_TR inside runAs, then a new runAs block with the re-queried user.
    @IsTest static void p3RunAsEnToTr() {
        Map<String,Object> o = new Map<String,Object>();
        User u = makeUser('en_US');
        o.put('admin_before', step());
        System.runAs(u) {
            o.put('runas_before', step());
            u.LocaleSidKey = 'tr_TR';
            update u;
            o.put('runas_after_update', step());
        }
        o.put('admin_between', step());
        User u2 = [SELECT Id, LocaleSidKey FROM User WHERE Id = :u.Id];
        System.runAs(u2) {
            o.put('runas2_requeried', step());
        }
        System.runAs(u) {
            o.put('runas3_same_object', step());
        }
        emit('n6.p3', o);
    }
    // Probe 3 variant: the update uses a separate User object (Id only), not the runAs variable.
    @IsTest static void p3bRunAsUpdateById() {
        Map<String,Object> o = new Map<String,Object>();
        User u = makeUser('en_US');
        System.runAs(u) {
            o.put('runas_before', step());
            update new User(Id = u.Id, LocaleSidKey = 'tr_TR');
            o.put('runas_after_update', step());
        }
        System.runAs(u) {
            o.put('runas2_same_object', step());
        }
        emit('n6.p3b', o);
    }
    // Probe 4: probe 3 in reverse, tr_TR to en_US.
    @IsTest static void p4RunAsTrToEn() {
        Map<String,Object> o = new Map<String,Object>();
        User u = makeUser('tr_TR');
        o.put('admin_before', step());
        System.runAs(u) {
            o.put('runas_before', step());
            u.LocaleSidKey = 'en_US';
            update u;
            o.put('runas_after_update', step());
        }
        o.put('admin_between', step());
        User u2 = [SELECT Id, LocaleSidKey FROM User WHERE Id = :u.Id];
        System.runAs(u2) {
            o.put('runas2_requeried', step());
        }
        System.runAs(u) {
            o.put('runas3_same_object', step());
        }
        emit('n6.p4', o);
    }
    // Probe 5: the update happens OUTSIDE runAs (admin updates U), then runAs(U) twice (stale object, re-queried).
    @IsTest static void p5AdminUpdatesThenRunAs() {
        Map<String,Object> o = new Map<String,Object>();
        User u = makeUser('en_US');
        System.runAs(u) { o.put('runas_before', step()); }
        update new User(Id = u.Id, LocaleSidKey = 'tr_TR');
        System.runAs(u) { o.put('runas_stale_object', step()); }
        User u2 = [SELECT Id, LocaleSidKey FROM User WHERE Id = :u.Id];
        System.runAs(u2) { o.put('runas_requeried', step()); }
        emit('n6.p5', o);
    }
}
