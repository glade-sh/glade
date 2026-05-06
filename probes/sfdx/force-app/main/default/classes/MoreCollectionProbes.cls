public class MoreCollectionProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'collections.list-sort') {
            return runListSort();
        } else if (probeId == 'collections.map-keyset') {
            return runMapKeyset();
        } else if (probeId == 'collections.set-equality') {
            return runSetEquality();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runListSort() {
        List<String> items = new List<String>{'banana', 'apple', 'cherry'};
        items.sort();
        return items[0] == 'apple';
    }

    private Object runMapKeyset() {
        Map<String, String> m = new Map<String, String>{'b' => '2', 'a' => '1'};
        Set<String> keys = m.keySet();
        return keys.size() == 2;
    }

    private Object runSetEquality() {
        Set<String> a = new Set<String>{'x', 'y'};
        Set<String> b = new Set<String>{'y', 'x'};
        return a.equals(b);
    }
}
