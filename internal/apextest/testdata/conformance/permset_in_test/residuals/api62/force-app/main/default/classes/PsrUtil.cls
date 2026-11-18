public with sharing class PsrUtil {
    private static Id cachedProfileId;
    private static Integer seq = 0;

    public static Id permSetId() {
        return [SELECT Id FROM PermissionSet WHERE Name = 'PsrSet' LIMIT 1].Id;
    }

    public static Id limitedProfileId() {
        if (cachedProfileId == null) {
            List<Profile> ps = [SELECT Id FROM Profile WHERE Name = 'Minimum Access - Salesforce' LIMIT 1];
            if (ps.isEmpty()) {
                ps = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
            }
            cachedProfileId = ps[0].Id;
        }
        return cachedProfileId;
    }

    public static User newUser(Id profileId, String tag, Integer idx) {
        seq++;
        return new User(
            Username = 'psr.' + tag + '.' + idx + '.' + seq + '.' + Math.abs(Math.mod(Crypto.getRandomInteger(), 1000000)) + '@example.com',
            Email = 'psr@example.invalid',
            LastName = 'Psr' + tag + idx,
            Alias = ('psr' + tag + idx).left(8),
            ProfileId = profileId,
            TimeZoneSidKey = 'America/Los_Angeles',
            LocaleSidKey = 'en_US',
            EmailEncodingKey = 'UTF-8',
            LanguageLocaleKey = 'en_US'
        );
    }

    // Reads PsrField__c two ways: Security.stripInaccessible(READABLE) and a
    // USER_MODE query. Returns strip=<...>,um=<...>.
    public static String flsRead(Id recId) {
        String strip;
        try {
            PsrObj__c r = [SELECT Id, PsrField__c FROM PsrObj__c WHERE Id = :recId];
            SObjectAccessDecision d = Security.stripInaccessible(AccessType.READABLE, new List<PsrObj__c>{ r });
            Set<String> removed = d.getRemovedFields().get('PsrObj__c');
            if (removed != null && removed.contains('PsrField__c')) {
                strip = 'stripped';
            } else if (d.getRecords().isEmpty()) {
                strip = 'noRows';
            } else {
                strip = 'ok:' + String.valueOf(d.getRecords()[0].get('PsrField__c'));
            }
        } catch (Exception e) {
            strip = 'ex:' + e.getTypeName();
        }
        String um;
        try {
            List<SObject> rows = Database.query(
                'SELECT PsrField__c FROM PsrObj__c WHERE Id = :recId', AccessLevel.USER_MODE
            );
            um = rows.isEmpty() ? 'noRows' : 'ok:' + String.valueOf(rows[0].get('PsrField__c'));
        } catch (Exception e) {
            um = 'ex:' + e.getTypeName();
        }
        return 'strip=' + strip + ',um=' + um;
    }

    public static String ura(Id userId, Id recId) {
        try {
            List<UserRecordAccess> rows = [
                SELECT RecordId, HasReadAccess, HasEditAccess, HasDeleteAccess
                FROM UserRecordAccess
                WHERE UserId = :userId AND RecordId = :recId
            ];
            if (rows.isEmpty()) {
                return 'none';
            }
            return 'r=' + rows[0].HasReadAccess + ',e=' + rows[0].HasEditAccess + ',d=' + rows[0].HasDeleteAccess;
        } catch (Exception e) {
            return 'ex:' + e.getTypeName();
        }
    }
}
