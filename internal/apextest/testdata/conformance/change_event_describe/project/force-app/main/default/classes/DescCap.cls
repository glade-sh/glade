public class DescCap {
    public static void run() {
        for (String n : new List<String>{'AccountChangeEvent','Parcel__ChangeEvent','Account','Parcel__c'}) {
            Schema.SObjectType t = Schema.getGlobalDescribe().get(n);
            if (t == null) { System.debug('CAP ' + n + ' MISSING'); continue; }
            Schema.DescribeSObjectResult d = t.getDescribe();
            List<String> keys = new List<String>(d.fields.getMap().keySet()); keys.sort();
            System.debug('CAP ' + n + ' fields(' + keys.size() + ')=' + String.join(keys, ','));
            List<String> kids = new List<String>();
            for (Schema.ChildRelationship cr : d.getChildRelationships()) {
                String c = String.valueOf(cr.getChildSObject());
                if (c.endsWith('ChangeEvent') || c.startsWith('Parcel')) kids.add(c + '.' + cr.getField() + ':' + cr.getRelationshipName());
            }
            kids.sort();
            System.debug('CAP ' + n + ' eventOrParcelChildren(' + kids.size() + ')=' + String.join(kids, ','));
        }
    }
}
