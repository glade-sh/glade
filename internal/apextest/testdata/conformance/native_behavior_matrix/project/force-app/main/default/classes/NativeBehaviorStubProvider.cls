global class NativeBehaviorStubProvider implements System.StubProvider {
    global Boolean matchesLiteral { get; private set; }
    global String parameterTypeName { get; private set; }

    global Object handleMethodCall(
        Object stubbedObject,
        String stubbedMethodName,
        Type returnType,
        List<Type> paramTypes,
        List<String> paramNames,
        List<Object> args
    ) {
        parameterTypeName = paramTypes.isEmpty() ? '' : paramTypes[0].getName();
        matchesLiteral = !paramTypes.isEmpty() && paramTypes[0] == List<Orchard__c>.class;
        return null;
    }
}
