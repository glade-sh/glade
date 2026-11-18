@IsTest(SeeAllData=true)
private without sharing class HidSystemTest {
    static User actor() {
        return [SELECT Id FROM User WHERE Alias = 'hid62' AND LastName = 'Hidden' LIMIT 1];
    }
    static Id target() {
        return [SELECT Id FROM HidRec__c WHERE Name = 'HiddenTarget' LIMIT 1].Id;
    }
    @IsTest
    static void listUpdate() {
        User u = actor();
        Id tid = target();
        String out;
        System.runAs(u) {
            Integer n = [SELECT COUNT() FROM HidRec__c];
            try {
                update new List<HidRec__c>{ new HidRec__c(Id = tid, Flag__c = '1') };
                out = 'n' + n + '|ok';
            } catch (DmlException e) {
                out = 'n' + n + '|EX|' + e.getDmlStatusCode(0);
            } catch (Exception e) {
                out = 'n' + n + '|EX|' + e.getTypeName();
            }
        }
        System.assert(false, out);
    }
    @IsTest
    static void dbUpdate() {
        User u = actor();
        Id tid = target();
        String out;
        System.runAs(u) {
            Integer n = [SELECT COUNT() FROM HidRec__c];
            try {
                Database.update(new HidRec__c(Id = tid, Flag__c = '2'));
                out = 'n' + n + '|ok';
            } catch (DmlException e) {
                out = 'n' + n + '|EX|' + e.getDmlStatusCode(0);
            } catch (Exception e) {
                out = 'n' + n + '|EX|' + e.getTypeName();
            }
        }
        System.assert(false, out);
    }
    @IsTest
    static void dbFalse() {
        User u = actor();
        Id tid = target();
        String out;
        System.runAs(u) {
            Integer n = [SELECT COUNT() FROM HidRec__c];
            Database.SaveResult sr = Database.update(new HidRec__c(Id = tid, Flag__c = '3'), false);
            out = sr.isSuccess() ? 'n' + n + '|ok' : 'n' + n + '|SAVE|' + sr.getErrors()[0].getStatusCode();
        }
        System.assert(false, out);
    }
}
