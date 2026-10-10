class P {
  public void out(String id, Object v) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();

class BaseValue {}
class NarrowValue extends BaseValue {
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
Object value='value'; Object r=new OverloadProbe().stringOnly(value); pq.out('C006',r);
