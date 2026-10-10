class P {
  public void out(String id, Object v) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
class OverloadProbe { public String choose(Integer value,Object label){return 'integer-object';} public String choose(Long value,String label){return 'long-string';} } Object r=new OverloadProbe().choose(1,'one'); pq.out('C004',r);
