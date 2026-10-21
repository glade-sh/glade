public class StubDispatchCallKey {
    public String methodName;
    public Integer arity;
    public StubDispatchCallKey(String methodName, Integer arity) {
        this.methodName = methodName;
        this.arity = arity;
    }
    public Boolean equals(Object other) {
        if (!(other instanceof StubDispatchCallKey)) return false;
        StubDispatchCallKey key = (StubDispatchCallKey)other;
        return methodName == key.methodName && arity == key.arity;
    }
    public Integer hashCode() {
        return 31;
    }
}
