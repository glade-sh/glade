@IsTest(SeeAllData=true)
private class HidNoDeclC {
    static User actor() {
        return HidUser.get();
    }
    static String who(Id tid, User u) {
        HidRec__c t = [SELECT OwnerId FROM HidRec__c WHERE Id = :tid];
        User o = [SELECT Profile.Name FROM User WHERE Id = :t.OwnerId];
        return (t.OwnerId == u.Id ? 'user' : 'admin') + '|' + o.Profile.Name;
    }
    static String listStatus(Id tid, User u, String tag) {
        String out;
        System.runAs(u) {
            Integer n = [SELECT COUNT() FROM HidRec__c WHERE Id = :tid];
            try {
                update new List<HidRec__c>{ new HidRec__c(Id = tid, Flag__c = tag) };
                out = 'n' + n + '|ok';
            } catch (DmlException e) {
                out = 'n' + n + '|EX|' + e.getDmlStatusCode(0);
            } catch (Exception e) {
                out = 'n' + n + '|EX|' + e.getTypeName();
            }
        }
        return out;
    }
    static String dbFalseStatus(Id tid, User u, String tag) {
        String out;
        System.runAs(u) {
            Integer n = [SELECT COUNT() FROM HidRec__c WHERE Id = :tid];
            Database.SaveResult sr = Database.update(new HidRec__c(Id = tid, Flag__c = tag), false);
            out = sr.isSuccess() ? 'n' + n + '|ok' : 'n' + n + '|SAVE|' + sr.getErrors()[0].getStatusCode();
        }
        return out;
    }
    static String payload(String tag, Id aId, Id bId, Id vId, User u) {
        String wa = who(aId, u);
        String wb = who(bId, u);
        String wv = who(vId, u);
        String ls = listStatus(aId, u, 'L');
        String ds = dbFalseStatus(bId, u, 'D');
        return tag + '|uprof=' + u.Profile.Name + '|ownA=' + wa + '|ownB=' + wb + '|ownV=' + wv + '|list=' + ls + '|db=' + ds;
    }

    @IsTest
    static void cBoth() {
        User u = actor();
        Id aId = [SELECT Id FROM HidRec__c WHERE Name = 'PreHiddenA' LIMIT 1].Id;
        Id bId = [SELECT Id FROM HidRec__c WHERE Name = 'PreHiddenB' LIMIT 1].Id;
        Id vId = [SELECT Id FROM HidRec__c WHERE Name = 'PreVisibleA' LIMIT 1].Id;
        System.assert(false, payload('c', aId, bId, vId, u));
    }
}
