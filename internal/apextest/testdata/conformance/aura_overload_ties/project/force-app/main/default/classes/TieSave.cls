public class TieSave { @AuraEnabled public static void save(String label, String value) { insert new Account(Name=label, Description=value); } }
