public class RecordingProvider implements System.StubProvider {
 public List<Map<String,Object>> calls=new List<Map<String,Object>>();
 public Object handleMethodCall(Object receiver,String member,Type resultType,List<Type> parameterTypes,List<String> parameterNames,List<Object> arguments) {
  List<String> types=new List<String>();
  for (Type item: parameterTypes) types.add(item==null?'null':item.getName());
  List<String> args=new List<String>();
  for (Object item: arguments) args.add(item==null?'null':String.valueOf(item));
  Map<String,Object> row=new Map<String,Object>{'member'=>member,'result'=>resultType==null?'null':resultType.getName(),'types'=>types,'names'=>parameterNames,'args'=>args};
  calls.add(row);
  return 'stub-'+member;
 }
}
