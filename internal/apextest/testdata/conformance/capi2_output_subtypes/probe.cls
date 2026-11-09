@IsTest private class Probe { @IsTest static void probe() {
 List<String> rows = new List<String>();
 for (String name : new List<String>{ /* cases.json names, batched */ }) {
  try { Object o = Type.forName(name).newInstance();
   Map<String,Object> m = (Map<String,Object>) JSON.deserializeUntyped(JSON.serialize(o));
   rows.add(name + '=' + String.valueOf(m.get('type')));
  } catch (Exception e) { rows.add(name + '=EX:' + e.getTypeName()); }
 }
 System.assert(false, 'ROW|' + String.join(rows, ';'));
} }
