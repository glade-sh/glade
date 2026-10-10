class P {
  public void out(String id, Object v) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
class BaseValue {} class NarrowValue extends BaseValue {} class LeafValue extends NarrowValue {} class OverloadProbe { public String choose(NarrowValue left,BaseValue right){return 'narrow-base';} public String choose(BaseValue left,LeafValue right){return 'base-leaf';} } Object r=new OverloadProbe().choose(new NarrowValue(),new LeafValue()); pq.out('C001',r);
