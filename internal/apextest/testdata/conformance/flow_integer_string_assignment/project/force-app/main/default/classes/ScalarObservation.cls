public class ScalarObservation {
 public static Map<String,Object> observe() {
  Map<String,Object> out=new Map<String,Object>();
  Account row=new Account(Name='Cedar',NumberOfEmployees=7,Description='before');
  try {insert row;out.put('inserted',[SELECT Description FROM Account WHERE Id=:row.Id].Description);row.Name='Birch';update row;out.put('updated',[SELECT Description FROM Account WHERE Id=:row.Id].Description);row.NumberOfEmployees=0;update row;out.put('zero',[SELECT Description FROM Account WHERE Id=:row.Id].Description);row.NumberOfEmployees=-12;update row;out.put('negative',[SELECT Description FROM Account WHERE Id=:row.Id].Description);row.NumberOfEmployees=null;update row;out.put('absent',[SELECT Description FROM Account WHERE Id=:row.Id].Description);out.put('exceptionType',null);}
  catch(Exception e){out.put('exceptionType',e.getTypeName());out.put('exceptionMessage',e.getMessage());}
  return out;
 }
}