public class ReflectionObservation {
 public static Map<String,Object> lookup(String name) {
  return new Map<String,Object>{'single'=>lookupMode(name,false),'emptyNamespace'=>lookupMode(name,true)};
 }
 public static Map<String,Object> lookupMode(String name,Boolean emptyNamespace) {
  Map<String,Object> row=new Map<String,Object>();
  try {
   Type found=emptyNamespace ? Type.forName('',name) : Type.forName(name);
   row.put('found',found!=null);
   if(found!=null) {
    row.put('name',found.getName());
    try {Object value=found.newInstance();row.put('newInstance',value!=null);}
    catch(Exception failure) {row.put('newInstanceErrorType',failure.getTypeName());row.put('newInstanceError',failure.getMessage());}
    try {Object value=JSON.deserialize('{}',found);row.put('deserialize',value!=null);}
    catch(Exception failure) {row.put('deserializeErrorType',failure.getTypeName());row.put('deserializeError',failure.getMessage());}
   }
  } catch(Exception failure) {row.put('errorType',failure.getTypeName());row.put('error',failure.getMessage());}
  return row;
 }
 public static Map<String,Object> inspect(Object value,String name) {
  Map<String,Object> row=new Map<String,Object>{'literalLookup'=>lookup(name)};
  try {SelectOption target=(SelectOption)value;row.put('castSucceeded',true);}
  catch(Exception failure) {
   row.put('castSucceeded',false);row.put('castType',failure.getTypeName());row.put('castMessage',failure.getMessage());
   String parsed=failure.getMessage().substringBetween('Invalid conversion from runtime type ',' to System.SelectOption');
   row.put('parsedName',parsed);row.put('parsedLookup',lookup(parsed));
  }
  return row;
 }
 public static Map<String,Object> observe() {
  return new Map<String,Object>{
   'privateRegular'=>inspect(ReflectionSubjects.privateValue(),'ReflectionSubjects.PrivateChild'),
   'publicRegular'=>inspect(ReflectionSubjects.publicValue(),'ReflectionSubjects.PublicChild'),
   'publicTop'=>inspect(new ReflectionProbeBase(),'ReflectionProbeBase')
  };
 }
}
