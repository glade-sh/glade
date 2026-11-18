public class SjAliasProbe62 {
  static String enc(String cs, List<Integer> cps){ try{ return EncodingUtil.urlEncode(String.fromCharArray(cps),cs); }catch(Exception e){ return e.getTypeName()+'|'+e.getMessage(); } }
  public static String run(){
    List<String> o=new List<String>();
    List<Integer> two=new List<Integer>{9312,8560};
    o.add('SA1 csWindows31J '+enc('csWindows31J',two));
    o.add('SA2 windows-932 '+enc('windows-932',two));
    o.add('SA3 MS_Kanji '+enc('MS_Kanji',two));
    o.add('SA4 cp932 '+enc('cp932',two));
    o.add('SA5 Windows-31J 57344='+enc('Windows-31J',new List<Integer>{57344}));
    o.add('SA6 Shift-JIS 57344='+enc('Shift-JIS',new List<Integer>{57344}));
    return String.join(o,' ## ');
  }
}
