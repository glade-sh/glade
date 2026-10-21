public class RecordDefaultListProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        result.put('defaultsTrue', summary(DefaultLedger__c.SObjectType.newSObject(null, true)));
        result.put('defaultsFalse', summary(DefaultLedger__c.SObjectType.newSObject(null, false)));
        result.put('defaultsOmitted', summary(DefaultLedger__c.SObjectType.newSObject()));
        result.put('ordinaryConstructor', summary(new DefaultLedger__c()));
        result.put('sumDescribeDefault', DefaultLedger__c.Total__c.getDescribe().getDefaultValue());
        result.put('countDescribeDefault', DefaultLedger__c.EntryCount__c.getDescribe().getDefaultValue());
        for (Integer i = 1; i <= 7; i++) {
            Map<String,Object> row = new Map<String,Object>();
            try {
                List<SObject> values;
                if (i == 1) {
                    values = (List<SObject>) Type.forName('List<Contact>').newInstance();
                } else if (i == 2) {
                    Object boxed = Type.forName('List<Contact>').newInstance();
                    values = (List<SObject>) boxed;
                } else if (i == 3) {
                    values = new List<Contact>();
                } else if (i == 4) {
                    values = (List<SObject>) Type.forName('List<SObject>').newInstance();
                } else if (i == 5) {
                    values = (List<SObject>) Type.forName('List<Account>').newInstance();
                    values.add(new Account(Name = 'List observation'));
                } else if (i == 6) {
                    values = (List<SObject>) Type.forName('list<contact>').newInstance();
                } else {
                    values = (List<SObject>) Type.forName('List<Contact>').newInstance();
                    values.add(new Account(Name = 'Wrong element control'));
                }
                row.put('token', String.valueOf(values.getSObjectType()));
                row.put('size', values.size());
            } catch (Exception error) {
                row.put('exceptionType', error.getTypeName());
                row.put('exceptionMessage', error.getMessage());
            }
            result.put('list' + i, row);
        }
        return result;
    }

    private static Map<String,Object> summary(SObject value) {
        return new Map<String,Object>{
            'sum' => value.get('Total__c'),
            'filteredSum' => value.get('IncludedTotal__c'),
            'count' => value.get('EntryCount__c'),
            'maximumDate' => value.get('Latest__c')
        };
    }
}
