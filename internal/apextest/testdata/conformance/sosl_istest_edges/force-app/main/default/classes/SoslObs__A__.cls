public class SoslObs__A__ {
    static String nameOf(SObject o) {
        Map<String, Schema.SObjectField> fm = o.getSObjectType().getDescribe().fields.getMap();
        if (fm.containsKey('name')) { return (String) o.get('Name'); }
        if (fm.containsKey('subject')) { return (String) o.get('Subject'); }
        return null;
    }
    public static String fmt(List<List<SObject>> lists) {
        List<String> parts = new List<String>();
        for (List<SObject> l : lists) {
            List<String> names = new List<String>();
            for (SObject o : l) {
                String n = nameOf(o);
                if (n != null && n.endsWith('qzx')) { names.add(n); }
            }
            names.sort();
            parts.add(String.join(names, ','));
        }
        return String.join(parts, ' ; ');
    }
    public static String sizes(List<List<SObject>> lists) {
        List<String> parts = new List<String>();
        for (List<SObject> l : lists) { parts.add('' + l.size()); }
        return String.join(parts, ',');
    }
    public static String fmtFind(Search.SearchResults sr, String objName) {
        List<String> parts = new List<String>();
        for (Search.SearchResult r : sr.get(objName)) {
            SObject o = r.getSObject();
            String n = nameOf(o);
            if (n != null && n.endsWith('qzx')) { parts.add(n + '=' + r.getSnippet()); }
        }
        parts.sort();
        return String.join(parts, ' ; ');
    }
}
