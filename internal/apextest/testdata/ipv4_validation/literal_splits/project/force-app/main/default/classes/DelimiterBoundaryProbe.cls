public class DelimiterBoundaryProbe {
 public static Map<String,Object> observe() {
  Map<String,Object> rows=new Map<String,Object>();
  List<String> patterns=new List<String>{'\\.','::','\\Q.\\E','[.]','(\\.)','(?i)x','(?=b)',''};
  List<String> inputs=new List<String>{'0.0.0.0','::north::::','..0..','0.0.0.0','a.b.','xXx','abc','ab'};
  for (Integer i=0;i<patterns.size();i++) {
   for (Integer splitLimit : new List<Integer>{-1,0,1,2,5}) {
    String key=String.valueOf(i)+'/'+String.valueOf(splitLimit);
    rows.put('String/'+key,inputs[i].split(patterns[i],splitLimit));
    rows.put('Pattern/'+key,Pattern.compile(patterns[i]).split(inputs[i],splitLimit));
   }
   rows.put('String/default/'+i,inputs[i].split(patterns[i]));
   rows.put('Pattern/default/'+i,Pattern.compile(patterns[i]).split(inputs[i]));
  }
  return rows;
 }
}
