public class CollectionWalkProbe {
    static Set<String> shared = new Set<String>();
    public class Entry { public List<String> tags = new List<String>{'seed'}; }
    public class Reader {
        public Map<String,Entry> entries = new Map<String,Entry>();
        public void unrelated(List<String> other) { other.add('outside'); }
        public void nested(List<String> tags) { tags.add('inside'); }
        public void replace() { entries.get('0').tags = new List<String>{'replacement'}; }
        public void instanceEdit() { entries.get('0').tags.add('instance'); }
    }
public static void staticEdit(Reader reader) { reader.entries.get('0').tags.add('static'); }
    public static Map<String,Object> observe() {
        Map<String,Object> out = new Map<String,Object>();
        Set<String> words = new Set<String>{'Alpha','beta'};
        Set<String> alias = words;
        out.put('stringAdd', words.add('Alpha'));
        out.put('stringCase', words.add('alpha'));
        out.put('stringNull', words.add(null));
        out.put('stringBulk', words.addAll(new List<String>{'beta','gamma'}));
        out.put('stringContains', words.contains('ALPHA'));
        out.put('stringAlias', alias.size());
        out.put('stringOrder', new List<String>(words));
        out.put('stringRemove', words.remove('beta'));
        out.put('stringRetain', words.retainAll(new Set<String>{'alpha',null,'gamma'}));
        out.put('stringAfter', new List<String>(words));
        Set<String> copy = words.clone(); words.clear(); copy.add('copy');
        out.put('clone', new List<String>(copy)); out.put('cleared', alias.size());
        Id shortId = (Id)'001000000000001';
        Id longId = (Id)'001000000000001AAA';
        Set<Id> ids = new Set<Id>{shortId};
        out.put('idDuplicate', ids.add(longId)); out.put('idContains', ids.contains('001000000000001AAA'));
        ids.addAll(new List<Id>{shortId, (Id)'001000000000002'}); ids.add(null);
        out.put('idSize', ids.size()); out.put('idRemove', ids.remove(longId));
        out.put('idRetain', ids.retainAll(new Set<Id>{(Id)'001000000000002'}));
        out.put('idAfter', new List<Id>(ids));
        Set<Decimal> numbers = new Set<Decimal>{1.0,1.00,2.0};
        out.put('decimalSize',numbers.size()); out.put('decimalContains', numbers.contains(1.000));
        numbers.addAll(new List<Decimal>{2.0,3.00}); numbers.remove(1.0);
        numbers.retainAll(new Set<Decimal>{1.00,3.00});
        out.put('decimalAfter', new List<Decimal>(numbers));
        Set<Object> mixed = new Set<Object>{1,1L,1.0,'1',null};
        out.put('mixedSize',mixed.size());
        shared.add('one'); shared.addAll(new Set<String>{'two','one'}); out.put('staticSet',shared.size());
        Reader reader = new Reader();
        for (Integer i=0;i<12;i++) { reader.entries.put(String.valueOf(i),new Entry()); }
        List<String> outside = new List<String>();
        List<String> nestedAlias = reader.entries.get('0').tags;
        reader.unrelated(outside); reader.nested(nestedAlias); reader.instanceEdit(); CollectionWalkProbe.staticEdit(reader);
        out.put('outside', outside); out.put('nested', nestedAlias); out.put('receiver',reader.entries.get('0').tags);
        reader.replace(); reader.unrelated(outside); reader.instanceEdit();
        out.put('replaced',reader.entries.get('0').tags); out.put('oldAlias',nestedAlias);
        return out;
    }
}
