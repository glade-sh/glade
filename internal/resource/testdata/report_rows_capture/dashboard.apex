Map<String,Schema.SObjectType> allTypes = Schema.getGlobalDescribe();
for (String obj : new List<String>{'Dashboard','Folder','ReportType'}) {
 if (!allTypes.containsKey(obj)) { System.debug('B6ROW|' + obj + '|MISSING'); continue; }
 Map<String,Schema.SObjectField> fields = allTypes.get(obj).getDescribe().fields.getMap();
 for (String fieldName : new List<String>{'Id','Title','Name','DeveloperName','FolderName','FolderId','Type','Format','NamespacePrefix'}) {
  if (fields.containsKey(fieldName)) System.debug('B6FIELD|' + obj + '|' + fieldName);
 }
 try {
  String query = 'SELECT Id';
  for (String fieldName : new List<String>{'Title','Name','DeveloperName','FolderName','Type','Format'}) if (fields.containsKey(fieldName)) query += ',' + fieldName;
  query += ' FROM ' + obj;
  for (SObject row : Database.query(query)) {
   String text = '';
   for (String fieldName : new List<String>{'Title','Name','DeveloperName','FolderName','Type','Format'}) if (fields.containsKey(fieldName)) text += '|' + fieldName + '=' + String.valueOf(row.get(fieldName));
   if (text.contains('B6')) System.debug('B6ROW|' + obj + text);
  }
 } catch(Exception e) { System.debug('B6ERROR|' + obj + '|' + e.getTypeName() + '|' + e.getMessage()); }
}
