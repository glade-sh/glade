@IsTest private class OverloadRejectedTest {
class OverloadProbe { public String sequence(Object value){return 'Object';} public String sequence(List<String> value){return 'List<String>';} } 
@IsTest static void C007(){
Object r=new OverloadProbe().sequence(null); 
System.assert(false,'P|C007|'+String.valueOf(r));
}
}
