public class MeadowTextProvider implements System.StubProvider {
    public Integer calls = 0;
    public Object handleMethodCall(Object target, String method, Type returnType,
            List<Type> argumentTypes, List<String> argumentNames, List<Object> arguments) {
        calls++;
        return method == 'toString' ? 'provider-text' : null;
    }
}
