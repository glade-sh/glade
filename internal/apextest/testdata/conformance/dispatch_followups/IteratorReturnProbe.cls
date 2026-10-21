public class IteratorReturnProbe {
    public class Cursor implements Iterator<Object> {
        private Integer position = 0;
        public Boolean hasNext() { return position == 0; }
        public Object next() { position++; return new Account(Name='Aster'); }
    }
    public class Source implements Iterable<SObject> {
        public Iterator<SObject> iterator() { return (Iterator<SObject>) new Cursor(); }
    }
    public class View implements Iterable<SObject> {
        private Iterable<SObject> source;
        public View(Iterable<SObject> source) { this.source = source; }
        public Iterator<SObject> iterator() { return source.iterator(); }
    }
    public static String observe() {
        Iterator<SObject> cursor = new View(new Source()).iterator();
        SObject row = cursor.next();
        return '' + row.get('Name') + '|' + cursor.hasNext();
    }
}
