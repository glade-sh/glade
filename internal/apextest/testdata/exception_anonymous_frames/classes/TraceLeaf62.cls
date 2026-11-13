public class TraceLeaf62 {
    public class InnerTraceException extends Exception {}
    public class OuterTraceException extends Exception {}

    public static void fail(String name) {
        if (name == 'null') {
            String value = null;
            Integer ignored = value.length();
        } else if (name == 'index') {
            List<String> values = new List<String>{'x'};
            String ignored = values[3];
        } else if (name == 'divide') {
            Integer zero = 0;
            Integer ignored = 5 / zero;
        } else if (name == 'cast') {
            Object value = 'not-a-number';
            Integer ignored = (Integer)value;
        } else if (name == 'query') {
            Account ignored = [SELECT Id FROM Account WHERE Id = null];
        } else if (name == 'required') {
            insert new Account();
        } else if (name == 'trigger') {
            insert new Account(Name='r459-trigger');
        } else if (name == 'chain') {
            Exception cause = new InnerTraceException('inner fixed');
            throw new OuterTraceException('outer fixed', cause);
        }
    }
}
