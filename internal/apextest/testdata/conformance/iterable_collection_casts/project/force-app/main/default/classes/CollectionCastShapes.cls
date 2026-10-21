public class CollectionCastShapes {
 public static String observe(String shape) {
 Object held;
 if(shape == 'list_records') held = new List<Account>{new Account(Name='Aster')};
 if(shape == 'set_records') held = new Set<Account>{new Account(Name='Aster')};
 if(shape == 'values_records') held = new Map<String,Account>{'one'=>new Account(Name='Aster')}.values();
 if(shape == 'list_objects') held = new List<Object>{new Account(Name='Aster')};
 if(shape == 'set_objects') held = new Set<Object>{new Account(Name='Aster')};
 if(shape == 'values_objects') held = new Map<String,Object>{'one'=>new Account(Name='Aster')}.values();
 if(shape == 'values_strings') held = new Map<String,String>{'one'=>'Aster'}.values();
 if(shape == 'list_strings') held = new List<String>{'Aster'};
 if(shape == 'set_strings') held = new Set<String>{'Aster'};
 if(shape == 'empty_list') held = new List<String>();
 if(shape == 'empty_set') held = new Set<String>();
 if(shape == 'map_control') held = new Map<String,Account>{'one'=>new Account(Name='Aster')};
 if(shape == 'scalar_control') held = 7;
 if(shape == 'null_control') held = null;
 String stage = 'cast';
 try {
  Iterable<SObject> view = (Iterable<SObject>)held;
  if(shape == 'list_strings' || shape == 'set_strings' || shape == 'values_strings') return 'cast|accepted';
  stage = 'iterate';
  Integer count = 0; String names = '';
  if(view != null) for(SObject row : view) { count++; names += String.valueOf(row.get('Name')); }
  Object boxed = view;
  return 'ok|' + count + '|' + names + '|' + (boxed instanceof List<Account>) + '|' + (boxed instanceof Set<Account>);
 } catch(Exception ex) { return stage + '|' + ex.getTypeName() + '|' + ex.getMessage(); }
 }
 public static Map<String,Object> observe() {
  Map<String,Object> out = new Map<String,Object>();
 out.put('list_records',observe('list_records'));
 out.put('set_records',observe('set_records'));
 out.put('values_records',observe('values_records'));
 out.put('list_objects',observe('list_objects'));
 out.put('set_objects',observe('set_objects'));
 out.put('values_objects',observe('values_objects'));
 out.put('values_strings',observe('values_strings'));
 out.put('list_strings',observe('list_strings'));
 out.put('set_strings',observe('set_strings'));
 out.put('empty_list',observe('empty_list'));
 out.put('empty_set',observe('empty_set'));
 out.put('map_control',observe('map_control'));
 out.put('scalar_control',observe('scalar_control'));
 out.put('null_control',observe('null_control'));
 return out;
 }
}
