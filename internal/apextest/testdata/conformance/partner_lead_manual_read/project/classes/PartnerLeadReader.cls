public with sharing class PartnerLeadReader {
 public static Integer countIds(Set<Id> ids) { return [SELECT COUNT() FROM Lead WHERE Id IN :ids]; }
 public static Integer queryIds(Set<Id> ids) { return [SELECT Id FROM Lead WHERE Id IN :ids].size(); }
}
