public class GardenBadScope {
 public static Integer observe() {
  IF(true) { Map<Id,Id> seeds = new Map<Id,Id>(); }
  return [SELECT Id FROM Account WHERE Id IN :seeds.values()].size();
 }
}
