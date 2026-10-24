public class CallbackShapeProvider implements System.StubProvider {
 public String observed;
 public Object handleMethodCall(Object stubbedObject, String stubbedMethodName, Type returnType, List<Type> parameterTypes, List<String> parameterNames, List<Object> arguments) {
  try { Date unexpected = (Date)arguments[0]; observed = 'converted'; }
  catch (System.TypeException problem) { observed = problem.getTypeName() + '|' + problem.getMessage(); }
  return null;
 }
}
