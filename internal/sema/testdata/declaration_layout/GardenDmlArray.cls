public class GardenDmlArray { public static Integer observe(){ Account[] seedlings=new List<Account>(); Database.SaveResult[] outcomes=Database.insert(seedlings,false); return outcomes.size(); } }
