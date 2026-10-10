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
try { Object r; Id value='001000000000001'; r=new OverloadProbe().bare(value,null); pq.out('R001', r); } catch (Exception e) { pq.err('R001', e); }
try { Object r; String value='001000000000001'; r=new OverloadProbe().bare(value,null); pq.out('R002', r); } catch (Exception e) { pq.err('R002', e); }
try { Object r; NarrowValue left=new NarrowValue(); BaseValue right=new LeafValue(); r=new OverloadProbe().choose(left,right); pq.out('R003', r); } catch (Exception e) { pq.err('R003', e); }
try { Object r; NarrowValue left=new NarrowValue(); BaseValue right=null; r=new OverloadProbe().choose(left,right); pq.out('R004', r); } catch (Exception e) { pq.err('R004', e); }
try { Object r; NarrowValue left=new NarrowValue(); BaseValue right=new LeafValue(); r=new OverloadProbe().choose(left,(LeafValue)right); pq.out('R005', r); } catch (Exception e) { pq.err('R005', e); }
try { Object r; r=new NarrowValue().chooseHeld(); pq.out('R006', r); } catch (Exception e) { pq.err('R006', e); }
try { Object r; Map<Id,List<String>> rows=null; r=new OverloadProbe().nullOnly(rows); pq.out('R007', r); } catch (Exception e) { pq.err('R007', e); }
try { Object r; List<String> value=null; r=new OverloadProbe().sequence(value); pq.out('R008', r); } catch (Exception e) { pq.err('R008', e); }
