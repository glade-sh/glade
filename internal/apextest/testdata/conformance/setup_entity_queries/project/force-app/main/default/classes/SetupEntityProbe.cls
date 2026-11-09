public class SetupEntityProbe {
    static String ex(Exception e) { return 'EX ' + e.getTypeName() + ':' + e.getMessage(); }
    public static Map<String, Object> observe() {
        Map<String, Object> o = new Map<String, Object>();
        String last;
        try {
            Set<String> names = new Set<String>();
            Integer pages = 0;
            do {
                List<EntityDefinition> gt = [SELECT QualifiedApiName FROM EntityDefinition WHERE QualifiedApiName > :last ORDER BY QualifiedApiName ASC LIMIT 2000];
                if (gt.isEmpty()) { break; }
                pages++;
                for (EntityDefinition e : gt) { names.add(e.QualifiedApiName); }
                last = gt[gt.size() - 1].QualifiedApiName;
                if (gt.size() < 2000) { break; }
            } while (pages < 20);
            o.put('gtNullHasAccount', names.contains('Account'));
            o.put('gtNullHasThing', names.contains('SpPrbThing__c'));
            o.put('gtNullNonEmpty', !names.isEmpty());
        } catch (Exception e) { o.put('gtNull', ex(e)); }
        try {
            List<String> r = new List<String>();
            for (EntityDefinition e : [SELECT QualifiedApiName, DeveloperName, IsCustomizable, IsCustomSetting, IsApexTriggerable FROM EntityDefinition WHERE QualifiedApiName LIKE 'SpPrb%' ORDER BY QualifiedApiName]) {
                if (e.QualifiedApiName.endsWith('__ChangeEvent') || e.QualifiedApiName.endsWith('__Share')) { continue; }
                r.add(e.QualifiedApiName + '/' + e.DeveloperName + '/c=' + e.IsCustomizable + '/cs=' + e.IsCustomSetting + '/t=' + e.IsApexTriggerable);
            }
            o.put('spPrbRows', r);
        } catch (Exception e) { o.put('spPrbRows', ex(e)); }
        try {
            List<String> r = new List<String>();
            for (EntityDefinition e : [SELECT QualifiedApiName, IsApexTriggerable FROM EntityDefinition WHERE QualifiedApiName IN ('Account', 'Contact', 'Opportunity', 'Lead', 'Case', 'Task', 'Event', 'User', 'Product2', 'Quote') ORDER BY QualifiedApiName]) {
                r.add(e.QualifiedApiName + '/t=' + e.IsApexTriggerable);
            }
            o.put('standardRows', r);
        } catch (Exception e) { o.put('standardRows', ex(e)); }
        try {
            List<String> r = new List<String>();
            for (EntityDefinition e : [SELECT DeveloperName FROM EntityDefinition WHERE IsCustomizable = TRUE AND QualifiedApiName LIKE '%__mdt' AND QualifiedApiName LIKE 'SpPrb%']) { r.add(e.DeveloperName); }
            o.put('customizableMdt', r);
        } catch (Exception e) { o.put('customizableMdt', ex(e)); }
        try {
            List<String> r = new List<String>();
            for (EntityDefinition e : [SELECT DeveloperName FROM EntityDefinition WHERE IsCustomSetting = TRUE AND QualifiedApiName LIKE 'SpPrb%' ORDER BY DeveloperName]) { r.add(e.DeveloperName); }
            o.put('customSettings', r);
        } catch (Exception e) { o.put('customSettings', ex(e)); }
        try {
            List<String> r = new List<String>();
            for (TabDefinition t : [SELECT Name, Label, IsCustom, SobjectName FROM TabDefinition WHERE Name LIKE 'SpPrb%']) { r.add(t.Name + '/' + t.Label + '/custom=' + t.IsCustom + '/' + t.SobjectName); }
            o.put('tabs', r);
        } catch (Exception e) { o.put('tabs', ex(e)); }
        try {
            List<String> r = new List<String>();
            for (AppMenuItem a : [SELECT Name, Label, Type FROM AppMenuItem WHERE Name = 'SpPrbApp']) { r.add(a.Name + '/' + a.Label + '/' + a.Type); }
            o.put('apps', r);
        } catch (Exception e) { o.put('apps', ex(e)); }
        try {
            o.put('standardAppExists', [SELECT COUNT() FROM AppMenuItem WHERE Type = 'TabSet' AND Name = 'Sales'] > 0);
        } catch (Exception e) { o.put('standardAppExists', ex(e)); }
        return o;
    }
}
