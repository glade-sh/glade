public with sharing class BarConstructions {
    private Object initial = new bar.Wrapper();
    public static Object make(Boolean live) {
        if (!live) { return null; }
        return new bar.Wrapper();
    }
    public static List<Object> collect(Boolean live) {
        List<Object> values = new List<Object>();
        if (!live) { return values; }
        Object holder = new bar.OtherWrapper();
        values.add(holder);
        values.add(new bar.LogBuilder());
        values.add(new bar.Wrapper('payload'));
        values.add(new List<bar.Wrapper>());
        values.add(new Map<String, bar.OtherWrapper>());
        List<Object> literal = new List<Object>{ new bar.Wrapper() };
        values.addAll(literal);
        for (bar.Wrapper item : new List<bar.Wrapper>()) { values.add(item); }
        values.add(new fooNs.LocalThing());
        values.add(new fooNs.Unknown());
        return values;
    }
}
