public class CollectionProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'collections.list-contains-null') {
            return runListContainsNull();
        } else if (probeId == 'collections.map-null-key') {
            return runMapNullKey();
        } else if (probeId == 'collections.set-contains-null') {
            return runSetContainsNull();
        } else if (probeId == 'collections.list-indexof-null') {
            return runListIndexOfNull();
        } else if (probeId == 'collections.map-remove-null') {
            return runMapRemoveNull();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runListContainsNull() {
        List<String> l = new List<String>();
        l.add(null);
        return l.contains(null);
    }

    private Object runMapNullKey() {
        Map<String, String> m = new Map<String, String>();
        m.put(null, 'val');
        return m.get(null);
    }

    private Object runSetContainsNull() {
        Set<String> s = new Set<String>();
        s.add(null);
        return s.contains(null);
    }

    private Object runListIndexOfNull() {
        List<String> l = new List<String>();
        l.add(null);
        return l.indexOf(null);
    }

    private Object runMapRemoveNull() {
        Map<String, String> m = new Map<String, String>();
        m.put(null, 'val');
        m.remove(null);
        return m.containsKey(null);
    }
}
