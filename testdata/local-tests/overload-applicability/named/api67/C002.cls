@IsTest private class CRT67eba70286ed33C002 {
class OverloadProbe { public String choose(String left,Object right){return 'string-object';} public String choose(Object left,String right){return 'object-string';} } 
@IsTest static void C002(){
Object r=new OverloadProbe().choose('left','right'); 
System.assert(false,'P|C002|'+String.valueOf(r));
}
}
