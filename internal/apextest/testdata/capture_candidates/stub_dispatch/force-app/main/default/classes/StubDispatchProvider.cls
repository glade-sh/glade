public class StubDispatchProvider implements System.StubProvider {
    public Object answer;
    public Boolean Verifying = false;
    public Boolean keyed = false;
    public Map<StubDispatchCallKey, Object> answers = new Map<StubDispatchCallKey, Object>();
    public List<StubDispatchTarget> proxies = new List<StubDispatchTarget>();
    public Object handleMethodCall(Object receiver, String methodName,
            Type returnType, List<Type> parameterTypes,
            List<String> parameterNames, List<Object> arguments) {
        StubDispatchProbe.providerCalls++;
        StubDispatchProbe.lastMethod = methodName;
        StubDispatchProbe.lastArity = arguments.size();
        if (keyed) {
            return answers.get(new StubDispatchCallKey(methodName, arguments.size()));
        }
        return answer;
    }
}
