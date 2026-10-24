public class JsonCycleObservation {
    public class Node { public Object next; public String label = 'root'; }
    public static String observe(String shape, String operation) {
        Map<String,Object> m = new Map<String,Object>(); m.put('self', m);
        List<Object> l = new List<Object>(); l.add(l);
        Object graph;
        if (shape == 'map') graph = m;
        if (shape == 'list') graph = l;
        if (shape == 'pair') {
            Map<String,Object> a = new Map<String,Object>();
            Map<String,Object> b = new Map<String,Object>();
            a.put('next', b); b.put('next', a); graph = a;
        }
        if (shape == 'class') { Node n = new Node(); n.next = n; graph = n; }
        if (shape == 'nested') {
            Map<String,Object> a = new Map<String,Object>();
            List<Object> b = new List<Object>(); a.put('items', b); b.add(a); graph = a;
        }
        if (shape == 'dag') { List<Object> leaf = new List<Object>{1, 'ok'}; graph = new List<Object>{leaf, leaf}; }
        if (shape == 'deep') {
            List<Object> root = new List<Object>(); List<Object> cursor = root;
            for (Integer i = 0; i < 128; i++) { List<Object> child = new List<Object>(); cursor.add(child); cursor = child; }
            cursor.add(1); graph = root;
        }
        Map<String,Object> row = new Map<String,Object>();
        try {
            String value;
            if (operation == 'serialize') value = JSON.serialize(graph);
            if (operation == 'pretty') value = JSON.serializePretty(graph);
            if (operation == 'suppress') value = JSON.serialize(graph, true);
            if (operation == 'generator') { System.JSONGenerator g = JSON.createGenerator(false); g.writeObject(graph); value = g.getAsString(); }
            if (operation == 'generatorField') { System.JSONGenerator g = JSON.createGenerator(false); g.writeStartObject(); g.writeObjectField('payload',graph); g.writeEndObject(); value = g.getAsString(); }
            if (operation == 'string') value = String.valueOf(graph);
            if (operation == 'toString') value = m.toString();
            if (operation == 'debug') { System.debug(graph); value = 'completed'; }
            if (operation == 'clone') { Map<String,Object> copy = m.clone(); value = String.valueOf(copy.get('self') === m); }
            if (operation == 'equalsSelf') value = String.valueOf(m == m);
            if (operation == 'equalsPair') { Map<String,Object> other = new Map<String,Object>(); other.put('self',other); value = String.valueOf(m == other); }
            if (operation == 'hash') value = String.valueOf(m.hashCode());
            if (operation == 'listEquals') value = String.valueOf(l == l);
            if (operation == 'listHash') value = String.valueOf(l.hashCode());
            row.put('value', value);
        } catch (Exception e) { row.put('type',e.getTypeName()); row.put('message',e.getMessage()); }
        return JSON.serialize(row);
    }
}
