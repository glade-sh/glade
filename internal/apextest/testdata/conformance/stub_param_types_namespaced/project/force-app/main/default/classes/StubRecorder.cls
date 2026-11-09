public class StubRecorder implements System.StubProvider {
    public static List<String> seen = new List<String>();
    public Object handleMethodCall(Object stubbedObject, String stubbedMethodName, Type returnType, List<Type> listOfParamTypes, List<String> listOfParamNames, List<Object> listOfArgs) {
        Type first = listOfParamTypes[0];
        seen.add(stubbedMethodName);
        seen.add('literalEq=' + (first == List<Thing__c>.class));
        seen.add('equalsEq=' + first.equals(List<Thing__c>.class));
        seen.add('valueOf=' + String.valueOf(first));
        seen.add('name=' + first.getName());
        return null;
    }
}
