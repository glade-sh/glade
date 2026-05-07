private class ProxyRecorder implements System.StubProvider {
  public Integer calls = 0;
  public String lastMethod;
  public String lastStringArg;
  public Boolean sawSameStub;
  public Object expectedStub;

  public Object handleMethodCall(Object stubbedObject, String stubbedMethodName, Type returnType, List<Type> listOfParamTypes, List<String> listOfParamNames, List<Object> listOfArgs) {
    calls++;
    lastMethod = stubbedMethodName;
    sawSameStub = expectedStub == null || expectedStub == stubbedObject;

    if (stubbedMethodName == 'add') {
      System.assertEquals('value', listOfParamNames.get(0));
      System.assertEquals(String.class, listOfParamTypes.get(0));
      lastStringArg = (String)listOfArgs.get(0);
      System.assertEquals(Void.class, returnType);
      return null;
    }

    if (stubbedMethodName == 'get2') {
      System.assertEquals('index', listOfParamNames.get(0));
      System.assertEquals(Integer.class, listOfParamTypes.get(0));
      lastStringArg = (String)listOfArgs.get(1);
      System.assertEquals(String.class, returnType);
      if (lastStringArg.contains('Hello')) {
        return 'hello';
      }
      return 'any';
    }

    return 'single';
  }
}
