public class CedarProbe {
    public static String afterSave(String objectName) {
        SObject r = Schema.getGlobalDescribe().get(objectName).newSObject();
        r.put('Name', 'a'); r.put('Cedar_Flag__c', true); r.put('Cedar_Log__c', 'old');
        Database.insert(r, AccessLevel.SYSTEM_MODE);
        SObject back = Database.query('SELECT Cedar_Log__c, Cedar_Text__c, Cedar_Flag__c FROM ' + objectName + ' WHERE Id = \'' + r.Id + '\' WITH SYSTEM_MODE');
        String insertSide = back.get('Cedar_Log__c') + '|' + back.get('Cedar_Text__c') + '|' + back.get('Cedar_Flag__c');
        back.put('Cedar_Flag__c', true); back.put('Cedar_Log__c', 'second');
        Database.update(back, AccessLevel.SYSTEM_MODE);
        SObject again = Database.query('SELECT Cedar_Log__c, Cedar_Text__c, Cedar_Flag__c FROM ' + objectName + ' WHERE Id = \'' + r.Id + '\' WITH SYSTEM_MODE');
        return 'insert=' + insertSide + ' update=' + again.get('Cedar_Log__c') + '|' + again.get('Cedar_Text__c') + '|' + again.get('Cedar_Flag__c');
    }
    public static String create(String flowName) {
        try {
            Flow.Interview.createInterview(flowName, new Map<String, Object>{ 'noteIn' => 'hi' }).start();
            Cedar_Response__c r = [SELECT Cedar_Participant__c, Cedar_Note__c FROM Cedar_Response__c WHERE Cedar_Version__c != null ORDER BY CreatedDate DESC, Id DESC LIMIT 1];
            return 'participantIsUser=' + (r.Cedar_Participant__c == UserInfo.getUserId()) + ' len=' + String.valueOf(r.Cedar_Participant__c).length() + ' note=' + r.Cedar_Note__c;
        } catch (Exception e) {
            return 'EXC ' + e.getTypeName() + ': ' + e.getMessage();
        }
    }
    public static String grouped() {
        String out = '';
        for (String soql : new List<String>{
            'SELECT COUNT(Id) c, toLabel(Cedar_Pick__c) lbl FROM Cedar_Report__c GROUP BY cedar_pick__c',
            'SELECT COUNT(Id) c, toLabel(Cedar_Pick__c) FROM Cedar_Report__c GROUP BY Cedar_Pick__c',
            'SELECT toLabel(Cedar_Pick__c) lbl FROM Cedar_Report__c GROUP BY Cedar_Pick__c',
            'SELECT COUNT(Id) c, toLabel(Cedar_Pick__c) lbl FROM Cedar_Report__c GROUP BY Cedar_Text__c'
        }) {
            try {
                List<String> parts = new List<String>();
                for (AggregateResult a : Database.query(soql)) {
                    parts.add(String.valueOf(a.getPopulatedFieldsAsMap()));
                }
                parts.sort();
                out += '[' + String.join(parts, ';') + '] ';
            } catch (Exception e) {
                out += '[EXC ' + e.getTypeName() + ': ' + e.getMessage() + '] ';
            }
        }
        return out;
    }
    public static void seedGrouped() {
        Database.insert(new List<Cedar_Report__c>{
            new Cedar_Report__c(Name='g1', Cedar_Pick__c='A_Val', Cedar_Text__c='t'),
            new Cedar_Report__c(Name='g2', Cedar_Pick__c='A_Val', Cedar_Text__c='t'),
            new Cedar_Report__c(Name='g3', Cedar_Pick__c='B_Val', Cedar_Text__c='u')
        }, AccessLevel.SYSTEM_MODE);
    }
}
