public class SjProbe67 {
  static String ex(Exception e){ String m=e.getMessage(); m=m==null?'null':m.replaceAll('\\s+',' '); return e.getTypeName()+'|'+m; }
  static String enc(String s,String cs){ try{ return EncodingUtil.urlEncode(s,cs); }catch(Exception e){ return ex(e); } }
  static String dec(String s,String cs){ try{ String r=EncodingUtil.urlDecode(s,cs); return r+'/'+(r.length()>0?String.valueOf(r.codePointAt(0)):'-'); }catch(Exception e){ return ex(e); } }
  public static String run(){
    List<String> o=new List<String>(); Integer k=0;
    String c1=String.fromCharArray(new List<Integer>{9312});
    for(String cs:new List<String>{'Shift-JIS','Shift_JIS','SJIS','Windows-31J','windows-31j','MS932','x-sjis','csShiftJIS','EUC-JP'}){
      k++; o.add('SJ'+k+' '+cs+' enc1='+enc(c1,cs)+' dec8740='+dec('%87%40',cs));
    }
    for(String cs:new List<String>{'Shift-JIS','Windows-31J'}){
      for(Integer cp:new List<Integer>{8786,8757,8544,13189,65374,8722,165,8212,8741}){
        k++; o.add('SJX'+k+' '+cs+' '+cp+'='+enc(String.fromCharArray(new List<Integer>{cp}),cs));
      }
    }
    try{ o.add('SJ6 '+EncodingUtil.convertToHex(Blob.valueOf(EncodingUtil.urlDecode('%87%40','Shift-JIS')))); }catch(Exception e){ o.add('SJ6 '+ex(e)); }
    return String.join(o,' ## ');
  }
}
