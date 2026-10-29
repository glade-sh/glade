public class WillowLookupProbe {
    public static Map<String, Object> observe() {
        Map<String, Object> rows = new Map<String, Object>();
        Account account = new Account(Name='Willow account');
        insert as system account;
        for (String mode : new List<String>{'single','missing','collection','record','variable','direct'}) {
            for (Boolean populated : new List<Boolean>{true,false}) {
                String key = mode + (populated ? '_set' : '_null');
                try {
                    Willow__c thing = new Willow__c(Name=mode, Account__c=populated ? account.Id : null);
                    insert as system thing;
                    String result;
                    if (mode == 'direct') {
                        Flow.Interview interview = Flow.Interview.createInterview('Willow_Child', new Map<String,Object>{'thingVar'=>thing});
                        interview.start();
                        result = (String)interview.getVariableValue('foundName');
                    } else {
                        result = [SELECT Result__c FROM Willow__c WHERE Id=:thing.Id WITH SYSTEM_MODE].Result__c;
                    }
                    rows.put(key, result == null ? '<null>' : result);
                } catch (Exception e) {
                    rows.put(key, 'ERROR|' + e.getTypeName() + '|' + e.getMessage());
                }
            }
        }
        return rows;
    }
}
