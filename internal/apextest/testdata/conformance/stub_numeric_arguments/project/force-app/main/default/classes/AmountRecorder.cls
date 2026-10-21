public class AmountRecorder implements System.StubProvider {
 public List<Object> calls = new List<Object>();
 public Object handleMethodCall(Object stubbedObject, String stubbedMethodName,
  Type returnType, List<Type> listOfParamTypes, List<String> listOfParamNames,
  List<Object> listOfArgs) {
  List<String> types = new List<String>();
  for (Type token : listOfParamTypes) types.add(token.getName());
  List<Object> expected;
  if (stubbedMethodName == 'format') {
   expected = new List<Object>{Decimal.valueOf('7')};
   if (listOfArgs.size() == 2) expected.add('pattern');
  } else if (stubbedMethodName == 'scalar') {
   expected = new List<Object>{7L, Double.valueOf('7')};
  } else {
   expected = new List<Object>{7};
  }
  String answer = 'stub-' + calls.size();
  calls.add(new Map<String,Object>{'method'=>stubbedMethodName,
   'types'=>types,'names'=>listOfParamNames,'returnType'=>returnType.getName(),
   'argumentsEqual'=>listOfArgs == expected, 'argumentsMethodEqual'=>listOfArgs.equals(expected)});
  return answer;
 }
}
