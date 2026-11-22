public with sharing class ShareProbeReader {
 public static Integer countRows(Id recordId) { return [SELECT count() FROM ShareProbe__c WHERE Id = :recordId]; }
}
