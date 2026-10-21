public class ScalarListView54 {
 public static Map<String,Object> inspect(List<Object> values) {return boxed(values);}
 private static Map<String,Object> boxed(Object values) {
  Map<String,Object> result=new Map<String,Object>{'size'=>((List<Object>)values).size()};
  try{Account row=(Account)values;result.put('accepted',true);result.put('resultNull',row==null);}
  catch(Exception e){result.put('accepted',false);result.put('exceptionType',e.getTypeName());result.put('exceptionMessage',e.getMessage());}
  return result;
 }
}