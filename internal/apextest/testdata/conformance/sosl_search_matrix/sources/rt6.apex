List<List<SObject>> r = Search.query('FIND {copper} RETURNING Account(Phone), Account(Name)');
String f = '';
try { Object a = r[0][0].get('Name'); f += 'N'; } catch (Exception e) { f += 'n'; }
try { Object a = r[0][0].get('Phone'); f += 'P'; } catch (Exception e) { f += 'p'; }
obs = r.size() + '|' + f;
