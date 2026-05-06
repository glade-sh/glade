private class GreeterProvider implements System.StubProvider {
  public Object handleMethodCall(Object stubbedObject, String stubbedMethodName, Type returnType, List<Type> listOfParamTypes, List<String> listOfParamNames, List<Object> listOfArgs) {
    System.assertEquals('greet', stubbedMethodName);
    return 'stubbed';
  }
}
