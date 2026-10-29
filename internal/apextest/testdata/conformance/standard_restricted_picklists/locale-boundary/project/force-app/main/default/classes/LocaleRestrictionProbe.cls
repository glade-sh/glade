public class LocaleRestrictionProbe {
 public static Map<String,Object> observe() {
  Map<String,Object> observation=new Map<String,Object>();
  Schema.DescribeFieldResult description=User.LocaleSidKey.getDescribe();
  List<Object> entries=new List<Object>();
  for (Schema.PicklistEntry entry:description.getPicklistValues()) if (!entry.getValue().contains('_')) entries.add(new Map<String,Object>{'value'=>entry.getValue(),'label'=>entry.getLabel(),'active'=>entry.isActive(),'defaultValue'=>entry.isDefaultValue()});
  observation.put('entries',entries);
  observation.put('restricted',description.isRestrictedPicklist());
  observation.put('totalEntryCount',description.getPicklistValues().size());
  for (String value : new List<String>{'NotAnAllowedValue','af','am','ar','az','be','bg','bn','bs','ca','cs','cy','da','de','dz','el','en','es','et','eu','fi','fr','ga','gu','haw','hi','hmn','hr','ht','hu','hy','in','is','it','iw','ja','ji','ka','kk','kl','km','kn','ko','ky','lb','lo','lt','lu','lv','mi','mk','ml','mr','ms','mt','my','ne','nl','no','pa','pl','pl_PL','ps','pt','rm','rn','ro','ru','sh','sk','sl','sm','so','sq','sr','sv','sw','ta','te','tg','th','ti','tl','tr','uk','ur','uz','vi','xh','xx','yo','zh','zu','zz'}) {
   Database.SaveResult result=Database.update(new User(Id=UserInfo.getUserId(),LocaleSidKey=value),false);
   List<Object> row=new List<Object>{result.isSuccess()};
   if(result.isSuccess()) row.add([SELECT LocaleSidKey FROM User WHERE Id=:UserInfo.getUserId()].LocaleSidKey);
   else if(value=='NotAnAllowedValue'){Database.Error problem=result.getErrors()[0];observation.put('error',new Map<String,Object>{'status'=>String.valueOf(problem.getStatusCode()),'message'=>problem.getMessage(),'fields'=>problem.getFields()});}
   observation.put(value,row);
  }
  return observation;
 }
}
