@IsTest
private class HidShareLocal {
    @IsTest
    static void listUpdate() {
        User u = HidSetup.run();
        Id tid = [SELECT Id FROM HidRec__c WHERE Name = 'HiddenTarget' LIMIT 1].Id;
        String out;
        System.runAs(u) {
            Integer n = [SELECT COUNT() FROM HidRec__c];
            try {
                update new List<HidRec__c>{ new HidRec__c(Id = tid, Flag__c = 'S') };
                out = 'n' + n + '|ok';
            } catch (DmlException e) {
                out = 'n' + n + '|EX|' + e.getDmlStatusCode(0);
            } catch (Exception e) {
                out = 'n' + n + '|EX|' + e.getTypeName();
            }
        }
        System.assert(false, out);
    }
}
