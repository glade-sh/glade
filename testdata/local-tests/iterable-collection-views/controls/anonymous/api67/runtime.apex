class P {
  public void out(String id, Object v) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
class CollectionViewProbe {
 public Object pass(Iterable<Object> values) { return values; }
}try { Object r; 
Set<String> original = new Set<String>{'Alpha'};
Iterable<Object> view = original;
Object boxed = view;
Iterable<Object> emptyView = new Set<String>();
Object emptyBoxed = emptyView;
r = String.valueOf(boxed instanceof Set<String>) + '|'
    + String.valueOf(boxed instanceof Set<Integer>) + '|'
    + String.valueOf(emptyBoxed instanceof Set<String>);
 pq.out('V001', r); } catch (Exception e) { pq.err('V001', e); }
try { Object r; 
List<String> original = new List<String>{'Alpha'};
Iterable<Object> view = original;
Object boxed = view;
Iterable<Object> emptyView = new List<String>();
Object emptyBoxed = emptyView;
r = String.valueOf(boxed instanceof List<String>) + '|'
    + String.valueOf(boxed instanceof List<Integer>) + '|'
    + String.valueOf(emptyBoxed instanceof List<String>);
 pq.out('V002', r); } catch (Exception e) { pq.err('V002', e); }
try { Object r; 
Iterable<Object> view = new Set<Integer>{1};
Object boxed = view;
Set<String> narrowed = (Set<String>)boxed;
r = narrowed.size();
 pq.out('V003', r); } catch (Exception e) { pq.err('V003', e); }
try { Object r; 
Iterable<Object> view = new Set<String>{'Alpha'};
Object boxed = view;
Set<String> narrowed = (Set<String>)boxed;
r = narrowed.size();
 pq.out('V004', r); } catch (Exception e) { pq.err('V004', e); }
try { Object r; 
Object result = new CollectionViewProbe().pass(new Set<String>{'Alpha'});
r = String.valueOf(result instanceof Set<String>) + '|'
    + String.valueOf(result instanceof Set<Integer>);
 pq.out('V005', r); } catch (Exception e) { pq.err('V005', e); }
try { Object r; 
Object result = new CollectionViewProbe().pass(new List<String>{'Alpha'});
r = String.valueOf(result instanceof List<String>) + '|'
    + String.valueOf(result instanceof List<Integer>);
 pq.out('V006', r); } catch (Exception e) { pq.err('V006', e); }
try { Object r; 
r = String.valueOf(new CollectionViewProbe().pass(new Set<String>()) instanceof Set<String>) + '|'
    + String.valueOf(new CollectionViewProbe().pass(new List<String>()) instanceof List<String>);
 pq.out('V007', r); } catch (Exception e) { pq.err('V007', e); }
try { Object r; 
Object result = new CollectionViewProbe().pass(new Set<Integer>{1});
Set<String> narrowed = (Set<String>)result;
r = narrowed.size();
 pq.out('V008', r); } catch (Exception e) { pq.err('V008', e); }
try { Object r; 
Object result = new CollectionViewProbe().pass(new List<Object>{'Alpha'});
r = String.valueOf(result instanceof List<String>);
 pq.out('V009', r); } catch (Exception e) { pq.err('V009', e); }
try { Object r; 
Object result = new CollectionViewProbe().pass(new List<String>{'Alpha'});
r = String.valueOf(result instanceof List<String>);
 pq.out('V010', r); } catch (Exception e) { pq.err('V010', e); }
try { Object r; 
Object result = new CollectionViewProbe().pass(new Set<String>{'Alpha'});
Set<String> narrowed = (Set<String>)result;
r = narrowed.size();
 pq.out('V011', r); } catch (Exception e) { pq.err('V011', e); }
