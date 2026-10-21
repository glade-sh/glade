public class IteratorProjectionProbe {
    public class Cursor implements Iterator<Object> {
        private Integer position = 0;
        public Boolean hasNext() { return position == 0; }
        public Object next() { position++; return 'Aster'; }
    }
    public class Source implements Iterable<Object> {
        public Iterator<Object> iterator() { return new Cursor(); }
    }
    public class View implements Iterable<String> {
        private Iterable<String> source;
        public View(Iterable<String> source) { this.source = source; }
        public Iterator<String> iterator() { return source.iterator(); }
    }
    public static String observe() {
        Iterator<String> cursor = new View((Iterable<String>) new Source()).iterator();
        return cursor.next() + '|' + cursor.hasNext();
    }
}
