@IsTest private class OverloadRejectedTest {

virtual class BaseValue {}
virtual class NarrowValue extends BaseValue {
 public BaseValue held;
 public String chooseHeld() {
  held = new LeafValue();
  return new OverloadProbe().choose(this, held);
 }
}
class LeafValue extends NarrowValue {}
class OverloadProbe {
 public String bare(Id value, Map<Id,List<String>> rows) { return 'id-map'; }
 public String bare(String value, NarrowValue rows) { return 'string-class'; }
 public String choose(NarrowValue left, LeafValue right) { return 'narrow-leaf'; }
 public String choose(BaseValue left, BaseValue right) { return 'base-base'; }
 public String nullOnly(Map<Id,List<String>> rows) { return 'map'; }
 public String nullOnly(NarrowValue rows) { return 'class'; }
 public String stringOnly(String value) { return 'string-only'; }
 public String sequence(Object value) { return 'Object'; }
 public String sequence(List<String> value) { return 'List<String>'; }
}

@IsTest static void C005(){
Object r=new OverloadProbe().nullOnly(null); 
System.assert(false,'P|C005|'+String.valueOf(r));
}
}
