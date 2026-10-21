public class GardenUpperBranch {
 public static Integer observe(Boolean gate) {
  Map<Id,Id> seeds = new Map<Id,Id>();
  List<Account> rows;
  IF(gate) {
   rows = [SELECT Id FROM Account WHERE Id IN :seeds.values()];
  }else{
   rows = [SELECT Id FROM Account WHERE Id IN :seeds.values()];
  }
  return rows.size();
 }
}
