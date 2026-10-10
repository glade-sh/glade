@IsTest private class CRT625d2c7ff4c537C004 {
class OverloadProbe { public String choose(Integer value,Object label){return 'integer-object';} public String choose(Long value,String label){return 'long-string';} } 
@IsTest static void C004(){
Object r=new OverloadProbe().choose(1,'one'); 
System.assert(false,'P|C004|'+String.valueOf(r));
}
}
