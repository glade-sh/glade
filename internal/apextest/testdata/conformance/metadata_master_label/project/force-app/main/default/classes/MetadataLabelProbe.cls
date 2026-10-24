public class MetadataLabelProbe {
public static Map<String,Object> staticLabel() { LabelRule__mdt row=[SELECT MasterLabel,DeveloperName FROM LabelRule__mdt WHERE DeveloperName='Alpha']; return new Map<String,Object>{'label'=>row.MasterLabel,'name'=>row.DeveloperName}; }
public static Map<String,Object> dynamicLabel() { SObject row=Database.query('SELECT MasterLabel FROM LabelRule__mdt WHERE MasterLabel = \'Visible Alpha\' LIMIT 1'); return new Map<String,Object>{'label'=>row.get('MasterLabel')}; }
public static Map<String,Object> emptyLabel() { List<LabelRule__mdt> rows=[SELECT MasterLabel FROM LabelRule__mdt WHERE DeveloperName='Absent']; return new Map<String,Object>{'count'=>rows.size()}; }
public static Map<String,Object> invalidField() { try { Database.query('SELECT MissingField FROM LabelRule__mdt'); return new Map<String,Object>{'rejected'=>false}; } catch(QueryException e) {return new Map<String,Object>{'rejected'=>true};} }
}
