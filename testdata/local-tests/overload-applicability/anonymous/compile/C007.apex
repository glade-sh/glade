class P {
  public void out(String id, Object v) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
class OverloadProbe { public String sequence(Object value){return 'Object';} public String sequence(List<String> value){return 'List<String>';} } Object r=new OverloadProbe().sequence(null); pq.out('C007',r);
