public class SchemaCollectionProbe {
    public class Parts {
        public String FirstName;
        public String Name;
    }
    public static void localNames() {
        Parts Account = new Parts();
        Account.FirstName = 'Aster';
        Parts Event = new Parts();
        Event.Name = 'Meadow';
    }
    public class Sink {
        public List<SObject> rows;
        public Sink(List<Opportunity> input) { rows = input; }
    }
    public static List<Opportunity> relay(List<SObject> rows) { return rows; }
    public static Integer accept(List<Opportunity> rows) { return rows.size(); }
    public static String observe(String shape) {
        try {
            if (shape == 'account_names') {
                List<String> names = new List<String>();
                for (Schema.SObjectField field : Account.SObjectType.getDescribe().fields.getMap().values()) {
                    if (field.getDescribe().isNameField()) names.add(field.getDescribe().getName());
                }
                names.sort();
                return String.join(names, ',');
            }
            if (shape == 'account_query') {
                Account row = new Account(Name='Owned meadow');
                insert row;
                Account loaded = [SELECT Name FROM Account WHERE Id = :row.Id];
                for (Schema.SObjectField field : Account.SObjectType.getDescribe().fields.getMap().values()) {
                    if (field.getDescribe().isNameField()) return String.valueOf(loaded.get(field));
                }
                return 'missing';
            }
            if (shape == 'event_name') {
                return String.valueOf(Event.SObjectType.getDescribe().fields.getMap().get('Name'));
            }
            if (shape == 'event_subject') {
                return String.valueOf(Event.SObjectType.getDescribe().fields.getMap().get('Subject'));
            }
            List<SObject> rows = new List<Account>{new Account(Name='Aster'),new Account(Name='Clover')};
            if (shape == 'constructor') {
                Sink sink = new Sink(rows);
                return sink.rows.size() + '|' + sink.rows[0].getSObjectType();
            }
            if (shape == 'parameter') return String.valueOf(accept(rows));
            if (shape == 'return') return String.valueOf(relay(rows).size());
            if (shape == 'assignment') {
                List<Opportunity> narrowed = rows;
                return String.valueOf(narrowed.size());
            }
            if (shape == 'cast') return String.valueOf(((List<Opportunity>)rows).size());
            return 'unknown';
        } catch (Exception error) {
            return error.getTypeName() + '|' + error.getMessage();
        }
    }
    public static Map<String,Object> observe() {
        Map<String,Object> observations = new Map<String,Object>();
        for (String shape : new List<String>{'account_names','account_query','event_name','event_subject','constructor','parameter','return','assignment','cast'}) {
            observations.put(shape, observe(shape));
        }
        return observations;
    }
}
