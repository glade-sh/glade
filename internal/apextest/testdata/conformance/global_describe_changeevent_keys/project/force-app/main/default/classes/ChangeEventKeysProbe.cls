public class ChangeEventKeysProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        Map<String, Schema.SObjectType> g = Schema.getGlobalDescribe();
        List<String> pkg = new List<String>(); List<String> proj = new List<String>(); Integer all = 0;
        for (String k : g.keySet()) {
            if (!k.endsWith('changeevent')) continue;
            all++;
            if (k.contains('v035')) pkg.add(k);
            if (k.contains('v35proj')) proj.add(k);
        }
        pkg.sort(); proj.sort();
        m.put('package', pkg);
        m.put('project', proj);
        List<String> std = new List<String>();
        for (String n : new List<String>{ 'account', 'contact', 'lead', 'opportunity', 'case', 'user', 'task' }) { std.add(n + ':' + g.containsKey(n + 'changeevent')); }
        m.put('standard', std);
        m.put('anyChangeEvent', all > 0);
        List<String> pkgObjects = new List<String>();
        for (String k : g.keySet()) { if (k.contains('v035')) pkgObjects.add(k); }
        pkgObjects.sort();
        m.put('allPackageKeys', pkgObjects);
        Schema.SObjectType ce = g.get('gladens004__v035env__changeevent');
        if (ce != null) {
            Schema.DescribeSObjectResult d = ce.getDescribe();
            m.put('pkgDescribe', d.getName() + '~' + d.getLabel() + '~' + d.isCustom() + '~' + d.isCreateable() + '~' + d.isQueryable());
        }
        Schema.SObjectType pce = g.get('v35proj__changeevent');
        if (pce != null) { Schema.DescribeSObjectResult d = pce.getDescribe(); m.put('projDescribe', d.getName() + '~' + d.getLabel() + '~' + d.isCustom() + '~' + d.isCreateable() + '~' + d.isQueryable()); }
        Schema.SObjectType ace = g.get('accountchangeevent');
        if (ace != null) { Schema.DescribeSObjectResult d = ace.getDescribe(); m.put('accDescribe', d.getName() + '~' + d.getLabel() + '~' + d.isCustom() + '~' + d.isCreateable() + '~' + d.isQueryable()); }
        return m;
    }
}
