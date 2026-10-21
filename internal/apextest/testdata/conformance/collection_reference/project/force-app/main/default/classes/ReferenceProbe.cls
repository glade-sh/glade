public class ReferenceProbe {
    public class Box {
        private final Object value;
        private final Boolean present;
        public Box(Object value) { this.value = value; this.present = true; }
        public Object get() { return value; }
        public Boolean isPresent() { return present; }
    }
    public abstract class Sink {
        private Box shared;
        public void attach(Object value) { shared = new Box(value); }
        public Box shared() { return shared; }
        public abstract void accept(Integer value);
    }
    public class Writer extends Sink {
        public override void accept(Integer value) { ((List<Integer>) shared().get()).add(value * 3); }
    }
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        List<Integer> values = new List<Integer>();
        Sink sink = new Writer();
        sink.attach(values);
        sink.accept(2); sink.accept(4);
        result.put('callbackSize', values.size());
        result.put('callbackValues', values);
        Object strings = new List<String>{'a','b'};
        result.put('stringList', strings instanceof List<Object>);
        Object children = Account.SObjectType.getDescribe().getChildRelationships();
        result.put('schemaList', children instanceof List<Object>);
        result.put('schemaIterable', children instanceof Iterable<Object>);
        if (children instanceof List<Object>) {
            List<Object> listValue = (List<Object>) children;
            result.put('childElement', listValue.isEmpty() || listValue[0] instanceof Schema.ChildRelationship);
        }
        Account record = new Account(Name='owned reference'); insert record;
        Id rowId = record.Id;
        List<SObject> matches = Database.query('SELECT Id FROM Account WHERE Id = :rowId');
        Box wrapped = new Box(matches);
        result.put('queryPresent', wrapped.isPresent());
        result.put('queryCount', ((List<SObject>) wrapped.get()).size());
        return result;
    }
}
