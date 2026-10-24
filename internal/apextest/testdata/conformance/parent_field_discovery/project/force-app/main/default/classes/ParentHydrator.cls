public class ParentHydrator {
 private final Map<String,Set<String>> missing=new Map<String,Set<String>>();
 private void remember(Map<String,Set<String>> fields,String key,String name) {
  Set<String> selected=fields.get(key);
  if(selected==null) {selected=new Set<String>();fields.put(key,selected);}
  selected.add(name);
 }
 public List<SObject> expand(List<SObject> rows,List<String> columns) {
  for(String column:columns) {
   List<String> parts=column.split('\\.');
   SObject cursor=rows[0];
   String last=parts.remove(parts.size()-1);
   try {for(String part:parts) cursor=cursor.getSObject(part);cursor.get(last);}
   catch(Exception failure) {remember(this.missing,rows[0].getSObjectType().toString(),column);}
  }
  Set<String> fields=this.missing.get(rows[0].getSObjectType().toString());
  if(fields==null || fields.isEmpty()) return rows;
  Set<Id> ids=new Map<Id,SObject>(rows).keySet();
  return Database.query('SELECT Id,'+String.join(new List<String>(fields),',')+' FROM Contact WHERE Id IN :ids ORDER BY Account.Name');
 }
 public Integer fieldCount() {Integer total=0;for(Set<String> fields:this.missing.values()) total+=fields.size();return total;}
}
