class P {
  public void out(String id, Object v) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
class OverloadProbe { public String choose(String left,Object right){return 'string-object';} public String choose(Object left,String right){return 'object-string';} } Object r=new OverloadProbe().choose(null,null); pq.out('C003',r);
