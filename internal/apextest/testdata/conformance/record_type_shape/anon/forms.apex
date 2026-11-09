Map<String,Object> m = new Map<String,Object>();
Map<Integer,List<Integer>> lists = new Map<Integer,List<Integer>>();
for (Integer n : new List<Integer>{9,10,11,12}) {
  List<Integer> l = new List<Integer>(); for (Integer i = 1; i <= n; i++) l.add(i);
  lists.put(n, l);
  Set<Integer> s = new Set<Integer>(l);
  System.debug('MARK|debugList' + n); System.debug(l);
  System.debug('MARK|debugSet' + n); System.debug(s);
  m.put('format' + n, String.format('{0}', new List<Object>{l}));
  m.put('formatSet' + n, String.format('{0}', new List<Object>{s}));
  m.put('valueOf' + n, String.valueOf(l));
  m.put('valueOfSet' + n, String.valueOf(s));
  m.put('concat' + n, '' + l);
  m.put('concatSet' + n, '' + s);
  m.put('nestedList' + n, String.valueOf(new List<Object>{l}));
  m.put('mapValue' + n, String.valueOf(new Map<String,Object>{'k' => l}));
  m.put('json' + n, JSON.serialize(l));
}
List<Integer> a = lists.get(11).clone(); List<Integer> b = lists.get(11).clone(); b[10] = 99;
m.put('jsonListKeys', JSON.serialize(new Map<List<Integer>,Integer>{a => 1, b => 2}));
m.put('jsonListKey12', JSON.serialize(new Map<List<Integer>,Integer>{lists.get(12) => 1}));
Set<String> keys = Schema.SObjectType.Account.fields.getMap().keySet();
m.put('keySetSize', keys.size());
m.put('keySetConcat', ('' + keys).length() < 400 ? ('' + keys) : ('' + keys).left(400));
m.put('keySetConcatEndsDots', ('' + keys).endsWith(', ...}'));
m.put('keySetValueOfEndsDots', String.valueOf(keys).endsWith(', ...}'));
System.debug('MARK|keySetDebug'); System.debug(keys);
System.debug('OBS|' + JSON.serialize(m));
