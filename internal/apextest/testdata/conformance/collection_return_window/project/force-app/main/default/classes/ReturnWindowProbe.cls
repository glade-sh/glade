public class ReturnWindowProbe {
    public class Box {
        public Object value;
        public Box(Object value) { this.value = value; }
    }
    public virtual class Pump {
        public Box target;
        public virtual void apply() { ((List<Integer>)target.value).add(7); }
    }
    public class InheritedPump extends Pump {
        public override void apply() { super.apply(); }
    }
    private static Box shared;
    private static List<Integer> noise = new List<Integer>();
    private static void throughAlias(Box input) {
        Box alias = input;
        ((List<Integer>)alias.value).add(2);
    }
    private static void appendValue(List<Integer> values) { values.add(3); }
    private static void throughCallee(Box input) { appendValue((List<Integer>)input.value); }
    private static void throughStatic() { ((List<Integer>)shared.value).add(4); }
    private static void throughMap(Box input) {
        ((Map<String,List<Integer>>)input.value).get('x').add(5);
    }
    private static void clearMap(Box input) {
        ((Map<String,List<Integer>>)input.value).clear();
        noise.add(8);
    }
    private static void invoke(Pump callback) { callback.apply(); }
    private static void unrelated(Box input) { noise.add(9); }
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        List<Integer> values = new List<Integer>{1};
        Box first = new Box(values);
        Box second = new Box(values);
        throughAlias(first);
        ((List<Integer>)second.value).add(11);
        result.put('alias', (List<Integer>)first.value);
        values = new List<Integer>{1};
        first = new Box(values); second = new Box(values);
        throughCallee(first);
        result.put('callee', (List<Integer>)second.value);
        values = new List<Integer>{1};
        shared = new Box(values); second = new Box(values);
        throughStatic();
        ((List<Integer>)second.value).add(14);
        result.put('static', (List<Integer>)shared.value);
        values = new List<Integer>{1};
        first = new Box(values); second = new Box(values);
        Pump callback = new InheritedPump(); callback.target = first;
        invoke(callback);
        ((List<Integer>)second.value).add(17);
        result.put('callback', (List<Integer>)callback.target.value);
        Map<String,List<Integer>> mapping = new Map<String,List<Integer>>{'x'=>new List<Integer>{1}};
        first = new Box(mapping); second = new Box(mapping);
        throughMap(first);
        ((Map<String,List<Integer>>)second.value).get('x').add(15);
        result.put('mapList', ((Map<String,List<Integer>>)first.value).get('x'));
        clearMap(first);
        result.put('mapClear', ((Map<String,List<Integer>>)second.value).size());
        values = new List<Integer>{1};
        first = new Box(values); second = new Box(values);
        unrelated(first);
        result.put('unrelated', (List<Integer>)second.value);
        return result;
    }
}
