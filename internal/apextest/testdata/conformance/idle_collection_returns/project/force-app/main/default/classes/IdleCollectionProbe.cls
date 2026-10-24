public class IdleCollectionProbe {
    public class Branch {
        public List<Integer> values;
        public Branch(List<Integer> input) { values = input; }
    }

    public class Reader {
        public List<Branch> branches = new List<Branch>();
        public Map<String, List<Integer>> groups = new Map<String, List<Integer>>();
        public Set<Integer> marks = new Set<Integer>{1};
        public Reader read() { return this; }
        public Reader nested() { return read(); }
        public Reader current { get { return this; } }
        public void append() { branches[0].values.add(8); }
        public Reader growing {
            get {
                branches[0].values.add(10);
                return this;
            }
        }
        public void growContainers() {
            groups.put('second', branches[0].values);
            marks.add(2);
        }
    }

    public static Reader forward(Object input) { return (Reader)input; }
    public static Reader forwardTyped(Reader input) { return input.read(); }
    public static void append(Reader input) { input.branches[0].values.add(9); }

    public static Map<String, Object> observe() {
        Reader reader = new Reader();
        List<Integer> shared = new List<Integer>{1};
        shared.add(2);
        reader.branches.add(new Branch(shared));
        for (Integer i = 1; i < 24; i++) {
            reader.branches.add(new Branch(new List<Integer>{i}));
        }
        reader.groups.put('first', shared);
        Map<String, List<Integer>> groupAlias = reader.groups;
        Set<Integer> setAlias = reader.marks;
        Integer instanceReads = 0;
        Integer staticReads = 0;
        Integer typedStaticReads = 0;
        Integer getterReads = 0;
        Integer nestedReads = 0;
        for (Integer i = 0; i < 12; i++) {
            instanceReads += reader.read().branches.size();
            staticReads += forward(reader).branches.size();
            typedStaticReads += forwardTyped(reader).branches.size();
            getterReads += reader.current.branches.size();
            nestedReads += reader.nested().branches.size();
        }
        shared.add(7);
        reader.append();
        append(reader);
        Reader grown = reader.growing;
        reader.growContainers();
        return new Map<String, Object>{
            'instanceReads' => instanceReads,
            'staticReads' => staticReads,
            'typedStaticReads' => typedStaticReads,
            'getterReads' => getterReads,
            'nestedReads' => nestedReads,
            'shared' => shared,
            'leaf' => grown.branches[0].values,
            'first' => groupAlias.get('first'),
            'second' => groupAlias.get('second'),
            'setSize' => setAlias.size(),
            'setContainsTwo' => setAlias.contains(2)
        };
    }
}
