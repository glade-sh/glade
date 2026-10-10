@IsTest private class CRT67eba70286ed33C003 {
class OverloadProbe { public String choose(String left,Object right){return 'string-object';} public String choose(Object left,String right){return 'object-string';} } 
@IsTest static void C003(){
Object r=new OverloadProbe().choose(null,null); 
System.assert(false,'P|C003|'+String.valueOf(r));
}
}
