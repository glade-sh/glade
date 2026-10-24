public class CedarEntryObserver {
    public static Map<String,Object> observe(String phase, String event, String mode) {
        Map<String,Object> results = new Map<String,Object>();
        List<String> operators = mode == 'changed'
            ? new List<String>{'NotEqualTo','GreaterThan','IsNull'}
            : new List<String>{'EqualTo','NotEqualTo','GreaterThan','IsNull','IsChanged'};
        for (String operation : operators) {
            for (String route : new List<String>{'apex_dml','database','bulk_dml'}) {
                String key = phase + '/' + event + '/' + mode + '/' + operation + '/' + route;
                List<CedarEntry__c> subjects = new List<CedarEntry__c>();
                Integer matches = route == 'bulk_dml' ? 2 : 1;
                for (Integer i = 0; i < matches + 1; i++) {
                    CedarEntry__c row = new CedarEntry__c(Name = key + '_' + i, Match__c = i < matches ? 'Match' : 'Control',
                        Group__c = phase + '_' + event + '_' + mode + '_' + operation,
                        Runs__c = 0, Value__c = 'blocked', Amount__c = 0, Link__c = 'https://' + operation + '-' + route + '-' + i + '.example.invalid');
                    if (event == 'create') qualify(row, operation);
                    subjects.add(row);
                }
                if (route == 'bulk_dml') insert subjects;
                else for (CedarEntry__c row : subjects) {
                    if (route == 'database') Database.insert(row, true);
                    else insert row;
                }
                Map<String,Object> stages = new Map<String,Object>();
                stages.put('created', effects(phase, subjects));
                if (event == 'update') {
                    subjects = reload(subjects);
                    for (CedarEntry__c row : subjects) qualify(row, operation);
                    save(subjects, route);
                    stages.put('updated', effects(phase, subjects));
                    subjects = reload(subjects);
                    for (CedarEntry__c row : subjects) row.Note__c = 'Unrelated update';
                    save(subjects, route);
                    stages.put('unchanged', effects(phase, subjects));
                }
                results.put(key, stages);
            }
        }
        return results;
    }
    private static void qualify(CedarEntry__c row, String operation) {
        if (operation == 'EqualTo' || operation == 'NotEqualTo' || operation == 'IsChanged') row.Value__c = 'ready';
        if (operation == 'GreaterThan') row.Amount__c = 10;
        if (operation == 'IsNull') row.Link__c = null;
    }
    private static void save(List<CedarEntry__c> subjects, String route) {
        if (route == 'bulk_dml') update subjects;
        else for (CedarEntry__c row : subjects) {
            if (route == 'database') Database.update(row, true);
            else update row;
        }
    }
    private static List<CedarEntry__c> reload(List<CedarEntry__c> subjects) {
        Set<Id> ids = new Set<Id>();
        for (CedarEntry__c row : subjects) ids.add(row.Id);
        Map<Id,CedarEntry__c> saved = new Map<Id,CedarEntry__c>([SELECT Id,Name,Group__c,Runs__c,Value__c,Amount__c,Link__c,Note__c FROM CedarEntry__c WHERE Id IN :ids]);
        List<CedarEntry__c> result = new List<CedarEntry__c>();
        for (CedarEntry__c row : subjects) result.add(saved.get(row.Id));
        return result;
    }
    private static List<Decimal> effects(String phase, List<CedarEntry__c> subjects) {
        List<Decimal> result = new List<Decimal>();
        if (phase == 'before') {
            for (CedarEntry__c row : reload(subjects)) result.add(row.Runs__c);
        } else {
            Set<Id> ids = new Set<Id>();
            for (CedarEntry__c row : subjects) ids.add(row.Id);
            Map<Id,Integer> counts = new Map<Id,Integer>();
            for (CedarEntryEffect__c effect : [SELECT Parent__c FROM CedarEntryEffect__c WHERE Parent__c IN :ids]) {
                Integer count = counts.containsKey(effect.Parent__c) ? counts.get(effect.Parent__c) : 0;
                counts.put(effect.Parent__c, count + 1);
            }
            for (CedarEntry__c row : subjects) result.add(counts.containsKey(row.Id) ? counts.get(row.Id) : 0);
        }
        return result;
    }
}
