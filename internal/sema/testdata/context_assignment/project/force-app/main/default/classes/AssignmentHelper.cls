public class AssignmentHelper {
 public static Database.SaveResult[] save(List<Specimen__c> records, Boolean partial) { return Database.insert(records, partial); }
 public static Database.SaveResult save(Specimen__c record, Boolean partial) { return Database.insert(record, partial); }
}
