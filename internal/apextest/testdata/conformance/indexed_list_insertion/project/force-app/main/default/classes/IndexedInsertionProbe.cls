public class IndexedInsertionProbe {
    public static Map<String, Object> observe() {
        List<String> returnBefore = new List<String>{'birch', 'elm'};
        List<String> returnAfter = new List<String>{'birch', 'elm'};
        List<String> breakBefore = new List<String>{'birch', 'elm'};
        List<String> breakAfter = new List<String>{'birch', 'elm'};
        insertAndReturn(returnBefore, 0);
        insertAndReturn(returnAfter, 1);
        insertAndBreak(breakBefore, 0);
        insertAndBreak(breakAfter, 1);
        return new Map<String, Object>{
            'return_before' => returnBefore,
            'return_after' => returnAfter,
            'break_before' => breakBefore,
            'break_after' => breakAfter
        };
    }

    private static void insertAndReturn(List<String> words, Integer position) {
        for (String word : words) {
            words.add(position, 'spruce');
            return;
        }
    }

    private static void insertAndBreak(List<String> words, Integer position) {
        for (String word : words) {
            words.add(position, 'spruce');
            break;
        }
    }
}
