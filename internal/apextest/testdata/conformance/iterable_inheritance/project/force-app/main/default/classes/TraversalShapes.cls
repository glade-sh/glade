public class TraversalShapes {
    public interface Root extends Iterable<Object> {}
    public interface Branch extends Root {}
    public interface Leaf extends Branch {}
    public class Values implements Leaf {
        public Iterator<Object> iterator() { return new List<Object>{4, 6, 8}.iterator(); }
    }
    public virtual class Base implements Iterable<Object> {
        public virtual Iterator<Object> iterator() { return new List<Object>{2, 3}.iterator(); }
    }
    public class Derived extends Base {}
    public static Map<String, Object> observe() {
        Leaf values = new Values();
        Integer sum = 0;
        for (Object item : values) { sum += (Integer)item; }
        Branch branch = new Values();
        Integer count = 0;
        for (Object item : branch) { count++; }
        Derived derived = new Derived();
        Integer inherited = 0;
        for (Object item : derived) { inherited += (Integer)item; }
        Iterable<Object> empty = new List<Object>();
        Integer emptyCount = 0;
        for (Object item : empty) { emptyCount++; }
        return new Map<String, Object>{'sum'=>sum, 'count'=>count, 'inherited'=>inherited, 'empty'=>emptyCount,
            'innerTypeFound'=>Type.forName('TraversalShapes.Values') != null};
    }
}
