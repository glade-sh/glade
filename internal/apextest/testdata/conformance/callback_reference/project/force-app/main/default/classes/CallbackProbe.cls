public class CallbackProbe {
    public class Holder {
        private final Object value;
        private final Boolean populated;
        private Holder() { populated = false; }
        private Holder(Object value) { this.value = value; populated = true; }
        public Object get() { return value; }
        public Boolean isPresent() { return populated; }
    }
    public static Holder empty() { return new Holder(); }
    public static Holder wrap(Object value) { return value == null ? empty() : new Holder(value); }
    public abstract class Handler {
        private Holder shared = empty();
        public void configure(Object shared) {
            if (shared instanceof Holder) { this.shared = (Holder) shared; }
            else { this.shared = wrap(shared); }
        }
        public Holder shared() { return shared; }
        public abstract Holder accept(Object value, Holder current, Integer index);
    }
    public class Writer extends Handler {
        public override Holder accept(Object value, Holder current, Integer index) {
            ((List<Integer>) shared().get()).add(((Integer) value) * 3);
            return empty();
        }
    }
    public class Driver {
        private Object shared;
        private List<Object> values;
        public Driver(List<Object> values) { this.values = values.clone(); }
        public Driver sharing(Object shared) { this.shared = shared; return this; }
        public void each(Handler callback) {
            callback.configure(shared);
            for (Integer i = 0; i < values.size(); i++) { callback.accept(values[i], empty(), i); }
        }
    }
    public static Map<String,Object> observe() {
        List<Integer> values = new List<Integer>();
        new Driver(new List<Object>{2,4}).sharing(values).each(new Writer());
        return new Map<String,Object>{'count'=>values.size(),'values'=>values};
    }
}
