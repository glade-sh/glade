public class ObjectAvailabilityRoutes {
 public static Map<String,Object> observe() {
  Map<String,Object> r = new Map<String,Object>();
  for (String n : new List<String>{'Quote','QuoteLineItem','WebStore','Account'}) {
   r.put(n,observeObject(n));
  }
  return r;
 }
 public static Map<String,Object> observeObject(String n) {
  Map<String,Object> r = new Map<String,Object>();
  String q='SELECT Id FROM '+n+' WHERE Id = :missing';
  String c='SELECT COUNT() FROM '+n+' WHERE Id = :missing';
  Id missing=null; Map<String,Object> b=new Map<String,Object>{'missing'=>missing};
  r.put('describeContains',Schema.getGlobalDescribe().containsKey(n));
  r.put('describeGet',Schema.getGlobalDescribe().get(n)!=null);
  r.put('typeEmpty',Type.forName('', 'Schema.'+n)!=null);
  r.put('typeNull',Type.forName(null, 'Schema.'+n)!=null);
  try { Database.query(q); r.put('qd',true); } catch (Exception e) { r.put('qd',e.getTypeName()); }
  try { Database.query(q,AccessLevel.USER_MODE); r.put('qu',true); } catch (Exception e) { r.put('qu',e.getTypeName()); }
  try { Database.query(q,AccessLevel.SYSTEM_MODE); r.put('qs',true); } catch (Exception e) { r.put('qs',e.getTypeName()); }
  try { Database.countQuery(c); r.put('cd',true); } catch (Exception e) { r.put('cd',e.getTypeName()); }
  try { Database.countQuery(c,AccessLevel.USER_MODE); r.put('cu',true); } catch (Exception e) { r.put('cu',e.getTypeName()); }
  try { Database.countQuery(c,AccessLevel.SYSTEM_MODE); r.put('cs',true); } catch (Exception e) { r.put('cs',e.getTypeName()); }
  try { Database.getQueryLocator(q); r.put('ld',true); } catch (Exception e) { r.put('ld',e.getTypeName()); }
  try { Database.getQueryLocator(q,AccessLevel.USER_MODE); r.put('lu',true); } catch (Exception e) { r.put('lu',e.getTypeName()); }
  try { Database.getQueryLocator(q,AccessLevel.SYSTEM_MODE); r.put('ls',true); } catch (Exception e) { r.put('ls',e.getTypeName()); }
  try { Database.getCursor(q); r.put('hd',true); } catch (Exception e) { r.put('hd',e.getTypeName()); }
  try { Database.getCursor(q,AccessLevel.USER_MODE); r.put('hu',true); } catch (Exception e) { r.put('hu',e.getTypeName()); }
  try { Database.getCursor(q,AccessLevel.SYSTEM_MODE); r.put('hs',true); } catch (Exception e) { r.put('hs',e.getTypeName()); }
  try { Database.getPaginationCursor(q); r.put('pd',true); } catch (Exception e) { r.put('pd',e.getTypeName()); }
  try { Database.getPaginationCursor(q,AccessLevel.USER_MODE); r.put('pu',true); } catch (Exception e) { r.put('pu',e.getTypeName()); }
  try { Database.getPaginationCursor(q,AccessLevel.SYSTEM_MODE); r.put('ps',true); } catch (Exception e) { r.put('ps',e.getTypeName()); }
  try { Database.queryWithBinds(q,b,AccessLevel.USER_MODE); r.put('qbu',true); } catch (Exception e) { r.put('qbu',e.getTypeName()); }
  try { Database.queryWithBinds(q,b,AccessLevel.SYSTEM_MODE); r.put('qbs',true); } catch (Exception e) { r.put('qbs',e.getTypeName()); }
  try { Database.countQueryWithBinds(c,b,AccessLevel.USER_MODE); r.put('cbu',true); } catch (Exception e) { r.put('cbu',e.getTypeName()); }
  try { Database.countQueryWithBinds(c,b,AccessLevel.SYSTEM_MODE); r.put('cbs',true); } catch (Exception e) { r.put('cbs',e.getTypeName()); }
  try { Database.getQueryLocatorWithBinds(q,b,AccessLevel.USER_MODE); r.put('lbu',true); } catch (Exception e) { r.put('lbu',e.getTypeName()); }
  try { Database.getQueryLocatorWithBinds(q,b,AccessLevel.SYSTEM_MODE); r.put('lbs',true); } catch (Exception e) { r.put('lbs',e.getTypeName()); }
  try { Database.getCursorWithBinds(q,b,AccessLevel.USER_MODE); r.put('hbu',true); } catch (Exception e) { r.put('hbu',e.getTypeName()); }
  try { Database.getCursorWithBinds(q,b,AccessLevel.SYSTEM_MODE); r.put('hbs',true); } catch (Exception e) { r.put('hbs',e.getTypeName()); }
  try { Database.getPaginationCursorWithBinds(q,b,AccessLevel.USER_MODE); r.put('pbu',true); } catch (Exception e) { r.put('pbu',e.getTypeName()); }
  try { Database.getPaginationCursorWithBinds(q,b,AccessLevel.SYSTEM_MODE); r.put('pbs',true); } catch (Exception e) { r.put('pbs',e.getTypeName()); }
  return r;
 }
}
