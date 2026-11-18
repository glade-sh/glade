public class MlProbe62 {
  static String ex(Exception e){ String m=e.getMessage(); m=m==null?'null':m.replaceAll('\\s+',' '); return e.getTypeName()+'|'+m; }
  static String vis(String s){ return s==null?'null':s.replace('\n','\\n').replace('\r','\\r').replace('\t','\\t'); }
  public static String run(){
    List<String> o=new List<String>();
    String a='''
first
second''';
    o.add('ML1 len='+a.length()+' ['+vis(a)+'] sub='+vis(a.substring(0,5)));
    String b='''
first
second
''';
    o.add('ML2 len='+b.length()+' ['+vis(b)+']');
    String c='''
one ''two'' \n tab\t end''';
    o.add('ML4 len='+c.length()+' ['+vis(c)+']');
    String e1='''
   indented
     more
   back
''';
    o.add('ML6 ['+vis(e1)+']');
    try{ o.add('ML3 ['+vis(String.format('''
Hi {0}
and {1}''',new List<Object>{'x',2}))+']'); }catch(Exception e){ o.add('ML3 '+ex(e)); }
    String g='''
  trailing  
x''';
    o.add('ML8 ['+vis(g)+']');
    String i='''
''';
    o.add('ML10 ['+vis(i)+'] len='+i.length());
    Map<String,Object> m=new Map<String,Object>{'who'=>'Ann','n'=>5,'d'=>Date.newInstance(2024,1,2),'dec'=>1.50,'nul'=>null,'b'=>true,'dt'=>Datetime.newInstanceGmt(2024,1,2,3,4,5)};
    String[] ts=new String[]{'Hello ${who}','Hi $${who}','Hi ${absent}','${n}|${d}|${dec}|${nul}|${b}','${dt}','${a.b}','${ }','${}','{!who}','{who}','${who','$who','${ who }','${WHO}','${who}${who}','${n+1}','$${','${who} $${n}','${who}$','a$b','${who.length()}'};
    Integer k=0;
    for(String t:ts){ k++; try{ o.add('TP'+k+' ['+vis(t)+'] -> '+vis(t.template(m))); }catch(Exception e){ o.add('TP'+k+' ['+vis(t)+'] '+ex(e)); } }
    try{ o.add('TPn '+vis('${who}'.template(null))); }catch(Exception e){ o.add('TPn '+ex(e)); }
    try{ o.add('TPe '+vis('${who}'.template(new Map<String,Object>()))); }catch(Exception e){ o.add('TPe '+ex(e)); }
    try{ o.add('TPs '+vis('${who}'.template(new Map<String,String>{'who'=>'S'}))); }catch(Exception e){ o.add('TPs '+ex(e)); }
    try{ o.add('TPi '+vis('${who}'.template(new Map<String,Integer>{'who'=>7}))); }catch(Exception e){ o.add('TPi '+ex(e)); }
    try{ o.add('TPsobj '+vis('${a.Name}'.template(new Map<String,Object>{'a'=>new Account(Name='AcmeX')}))); }catch(Exception e){ o.add('TPsobj '+ex(e)); }
    return String.join(o,' ## ');
  }
}
