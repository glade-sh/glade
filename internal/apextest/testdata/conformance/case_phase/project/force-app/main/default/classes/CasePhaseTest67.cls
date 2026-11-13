@IsTest public class CasePhaseTest67 {
 @IsTest static void observe() {
  Id profileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id;
  String nonce=String.valueOf(DateTime.now().getTime());
  User owner=new User(Username='case.owner.'+nonce+'@example.invalid',Alias='cown',Email='owner@example.invalid',LastName='CaseOwner',ProfileId=profileId,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
  User reader=new User(Username='case.reader.'+nonce+'@example.invalid',Alias='cread',Email='reader@example.invalid',LastName='CaseReader',ProfileId=profileId,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
  insert new List<User>{owner,reader};
  Group target=[SELECT Id FROM Group WHERE DeveloperName='CasePhaseReaders' AND Type='Regular' LIMIT 1];
  insert new GroupMember(GroupId=target.Id,UserOrGroupId=reader.Id);
  CasePhaseCapture.readerId=reader.Id; CasePhaseCapture.submitterId=owner.Id; CasePhaseCapture.escalationQueueId=[SELECT Id FROM Group WHERE DeveloperName='CaseEscalationTarget' AND Type='Queue' LIMIT 1].Id;
  CasePhaseCapture.afterInsertAccess='unset'; CasePhaseCapture.afterInsertShareRows='unset'; CasePhaseCapture.escalationDuringState='unset';
  Id shareCaseId; Id noShareCaseId; Id escalationCaseId;
  System.runAs(owner) {
   Case shareCase=new Case(Subject='NATIVE_CASE_SHARE_PROBE',Status='New',Origin='Web'); insert shareCase; shareCaseId=shareCase.Id;
   Case noShareCase=new Case(Subject='NATIVE_CASE_NO_SHARE',Status='New',Origin='Web'); insert noShareCase; noShareCaseId=noShareCase.Id;
   Case escalationCase=new Case(Subject='NATIVE_CASE_ESCALATION_PROBE',Status='New',Origin='Web'); insert escalationCase; escalationCaseId=escalationCase.Id;
  }
  Integer readerShareCount; Integer readerNoShareCount;
  List<UserRecordAccess> matchAccess=[SELECT RecordId,HasReadAccess FROM UserRecordAccess WHERE UserId=:reader.Id AND RecordId=:shareCaseId];
  List<UserRecordAccess> controlAccess=[SELECT RecordId,HasReadAccess FROM UserRecordAccess WHERE UserId=:reader.Id AND RecordId=:noShareCaseId];
  String afterMatch=matchAccess.isEmpty()?'no-row':String.valueOf(matchAccess[0].HasReadAccess);
  String afterControl=controlAccess.isEmpty()?'no-row':String.valueOf(controlAccess[0].HasReadAccess);
  System.runAs(reader) {
   readerShareCount=[SELECT COUNT() FROM Case WHERE Id=:shareCaseId];
   readerNoShareCount=[SELECT COUNT() FROM Case WHERE Id=:noShareCaseId];
  }
  List<Case> escalationRows=[SELECT OwnerId,IsEscalated FROM Case WHERE Id=:escalationCaseId];
  String escalationState=escalationRows.isEmpty()?'no-row':String.valueOf(escalationRows[0].IsEscalated)+':'+CasePhaseCapture.ownerKind(escalationRows[0].OwnerId);
  String observed='during:'+CasePhaseCapture.afterInsertAccess+':'+CasePhaseCapture.afterInsertShareRows+':'+CasePhaseCapture.escalationDuringState+'|after:'+afterMatch+','+afterControl+'|testQuery:'+readerShareCount+','+readerNoShareCount+'|escalation:'+escalationState;
  System.assertEquals('EXPECTED',observed);
 }
}
