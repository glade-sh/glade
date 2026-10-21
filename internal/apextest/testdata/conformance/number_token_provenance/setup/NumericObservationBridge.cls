public class NumericObservationBridge {
 public static Map<String,Object> inspect(Object v){
  Map<String,Object> p=new Map<String,Object>{'null'=>v==null,'string'=>String.valueOf(v),'json'=>JSON.serialize(v)};
  try{Integer x=(Integer)v;p.put('Integer',new List<Object>{'ok',x});}catch(Exception e){p.put('Integer',new List<Object>{e.getTypeName(),e.getMessage()});}
  try{Long x=(Long)v;p.put('Long',new List<Object>{'ok',x});}catch(Exception e){p.put('Long',new List<Object>{e.getTypeName(),e.getMessage()});}
  try{Decimal x=(Decimal)v;p.put('Decimal',new List<Object>{'ok',x});}catch(Exception e){p.put('Decimal',new List<Object>{e.getTypeName(),e.getMessage()});}
  try{Double x=(Double)v;p.put('Double',new List<Object>{'ok',x});}catch(Exception e){p.put('Double',new List<Object>{e.getTypeName(),e.getMessage()});}
  return p;
 }
}