class P {
  public void out(String id, Object v) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
class CollectionViewProbe {
 public Object pass(Iterable<Object> values) { return values; }
}try { Object r; Object value=new CollectionViewProbe().pass(new Set<String>{'Alpha'}); r=String.valueOf(value instanceof Set<String>); pq.out('R001', r); } catch (Exception e) { pq.err('R001', e); }
try { Object r; Object value=new CollectionViewProbe().pass(new List<String>{'Alpha'}); r=String.valueOf(value instanceof List<String>); pq.out('R002', r); } catch (Exception e) { pq.err('R002', e); }
try { Object r; Account row=new Account(Name='Alpha'); insert row; Object wanted=new CollectionViewProbe().pass(new Set<String>{'Alpha'}); Map<String,Object> bindings=new Map<String,Object>{'wanted'=>wanted}; r=Database.queryWithBinds('SELECT Id FROM Account WHERE Name IN :wanted',bindings,AccessLevel.USER_MODE).size(); pq.out('R003', r); } catch (Exception e) { pq.err('R003', e); }
try { Object r; Account row=new Account(Name='Alpha'); insert row; Object wanted=new CollectionViewProbe().pass(new List<String>{'Alpha'}); Map<String,Object> bindings=new Map<String,Object>{'wanted'=>wanted}; r=Database.queryWithBinds('SELECT Id FROM Account WHERE Name IN :wanted',bindings,AccessLevel.USER_MODE).size(); pq.out('R004', r); } catch (Exception e) { pq.err('R004', e); }
try { Object r; Account row=new Account(Name='Alpha'); insert row; Object wanted=new CollectionViewProbe().pass(new Set<Object>{'Alpha'}); Map<String,Object> bindings=new Map<String,Object>{'wanted'=>wanted}; r=Database.queryWithBinds('SELECT Id FROM Account WHERE Name IN :wanted',bindings,AccessLevel.USER_MODE).size(); pq.out('R005', r); } catch (Exception e) { pq.err('R005', e); }
try { Object r; Account row=new Account(Name='Alpha'); insert row; Object wanted=new CollectionViewProbe().pass(new List<Object>{'Alpha'}); Map<String,Object> bindings=new Map<String,Object>{'wanted'=>wanted}; r=Database.queryWithBinds('SELECT Id FROM Account WHERE Name IN :wanted',bindings,AccessLevel.USER_MODE).size(); pq.out('R006', r); } catch (Exception e) { pq.err('R006', e); }
try { Object r; Object value=new CollectionViewProbe().pass(new Set<Integer>{1}); Set<String> narrowed=(Set<String>)value; r=narrowed.size(); pq.out('R007', r); } catch (Exception e) { pq.err('R007', e); }
try { Object r; Object value=new CollectionViewProbe().pass(new Set<String>()); r=String.valueOf(value instanceof Set<String>); pq.out('R008', r); } catch (Exception e) { pq.err('R008', e); }
try { Object r; Object value=new CollectionViewProbe().pass(new List<String>()); r=String.valueOf(value instanceof List<String>); pq.out('R009', r); } catch (Exception e) { pq.err('R009', e); }
