public class TemporarySetCursorProbe {
    public static Map<String, Object> observe() {
        Map<String, Object> observation = new Map<String, Object>();
        Set<String> changing = new Set<String>{'birch', 'elm'};
        String chosen = changing.iterator().next();
        Boolean removed = changing.remove(chosen);
        Boolean added = changing.add('spruce');
        observation.put('temporary_remove_add', new Map<String, Object>{
            'removed' => removed,
            'added' => added,
            'size' => changing.size(),
            'contains_removed' => changing.contains(chosen),
            'contains_added' => changing.contains('spruce')
        });

        Set<String> growing = new Set<String>{'birch', 'elm'};
        String retained = growing.iterator().next();
        Boolean grew = growing.add('spruce');
        observation.put('temporary_add_only', new Map<String, Object>{
            'added' => grew,
            'size' => growing.size(),
            'contains_original' => growing.contains(retained),
            'contains_added' => growing.contains('spruce')
        });
        observation.put('stored_remove_add', observeStored());
        return observation;
    }

    public static Map<String, Object> observeStored() {
        Set<String> changing = new Set<String>{'birch', 'elm'};
        Iterator<String> cursor = changing.iterator();
        String chosen = cursor.next();
        Boolean removed = changing.remove(chosen);
        Boolean added = changing.add('spruce');
        return new Map<String, Object>{
            'removed' => removed,
            'added' => added,
            'size' => changing.size(),
            'contains_removed' => changing.contains(chosen),
            'contains_added' => changing.contains('spruce')
        };
    }
}
