public class TextJoiner {
    private static List<String> stringify(List<Object> values) {
        List<String> result = new List<String>();
        for (Object value : values) { result.add(String.valueOf(value)); }
        return result;
    }
    public class Segment {
        private List<Object> elements = new List<Object>{7, 'blue'};
        public String render() { return String.join(stringify(elements), ','); }
        public String localRender() { List<String> values = stringify(elements); return String.join(values, ','); }
    }
    public static String fieldCall() { return new Segment().render(); }
    public static String localCall() { return new Segment().localRender(); }
}
