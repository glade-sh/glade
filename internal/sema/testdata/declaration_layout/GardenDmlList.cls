public class GardenDmlList {
 public static Integer observe() {
  List<Account> seedlings = new List<Account>();
  Database.SaveResult[] outcomes = Database.update(seedlings,false);
  return outcomes.size();
 }
}
