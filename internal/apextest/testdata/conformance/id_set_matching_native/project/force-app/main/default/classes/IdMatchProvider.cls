public class IdMatchProvider implements System.StubProvider {
    public Object expected;
    public Object actual;
    public List<Type> signature;
    public Object handleMethodCall(Object target, String method, Type output, List<Type> inputs, List<String> names, List<Object> args) {
        actual = args[0];
        signature = inputs;
        return null;
    }
}
