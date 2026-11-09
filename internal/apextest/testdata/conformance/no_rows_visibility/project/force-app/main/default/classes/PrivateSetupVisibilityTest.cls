@IsTest public class PrivateSetupVisibilityTest {
 @TestSetup static void seed() { insert new List<PrivateProbe__c>{new PrivateProbe__c(Name='A'),new PrivateProbe__c(Name='B'),new PrivateProbe__c(Name='C')}; }
 static User makeUser(String suffix, Id role) {
  return new User(Username='probe.'+suffix+'.'+UserInfo.getOrganizationId()+'@example.invalid', Alias='prb', Email='probe@example.invalid', LastName='Probe', ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id, UserRoleId=role, TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
 }
 @IsTest static void visibility() {
  User reader=makeUser('reader',null); insert reader;
  insert new PermissionSetAssignment(AssigneeId=reader.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1].Id);
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(reader) {
   result.put('privateCount',[SELECT COUNT() FROM PrivateProbe__c]);
   result.put('privateLocator',Database.getQueryLocator('SELECT Id FROM PrivateProbe__c LIMIT 1').iterator().hasNext());
   try { PrivateProbe__c one=[SELECT Id FROM PrivateProbe__c LIMIT 1]; result.put('singleton','found'); } catch(QueryException e){result.put('singleton',e.getTypeName());}
  }
  result.put('ownerLocator1',Database.getQueryLocator('SELECT Id FROM PrivateProbe__c ORDER BY Name LIMIT 1').iterator().hasNext());
  Database.QueryLocatorIterator it=Database.getQueryLocator('SELECT Id FROM PrivateProbe__c ORDER BY Name LIMIT 2').iterator(); Integer n=0; while(it.hasNext()){it.next(); n++;} result.put('ownerLocator2',n);
  System.assert(false,'OBSERVED|visibility|'+JSON.serialize(result));
 }
 @IsTest static void hierarchy() {
  Map<String,Id> roles=new Map<String,Id>(); for(UserRole r:[SELECT Id,DeveloperName FROM UserRole WHERE DeveloperName IN ('ProbeParent','ProbeChild')]) roles.put(r.DeveloperName,r.Id);
  User parent=makeUser('parent',roles.get('ProbeParent')); User child=makeUser('child',roles.get('ProbeChild')); User other=makeUser('other',null); insert new List<User>{parent,child,other};
  Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1].Id; insert new List<PermissionSetAssignment>{new PermissionSetAssignment(AssigneeId=parent.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=child.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=other.Id,PermissionSetId=ps)};
  System.runAs(child) { insert new PrivateProbe__c(Name='ChildOwned'); }
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(parent){result.put('parent',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='ChildOwned']);}
  System.runAs(other){result.put('unrelated',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='ChildOwned']);}
  System.assert(false,'OBSERVED|hierarchy|'+JSON.serialize(result));
 }
 @IsTest static void hierarchyDepth() {
  Map<String,Id> roles=new Map<String,Id>(); for(UserRole r:[SELECT Id,DeveloperName FROM UserRole WHERE DeveloperName IN ('ProbeParent','ProbeChild','ProbeGrandchild')]) roles.put(r.DeveloperName,r.Id);
  User parent=makeUser('deepParent',roles.get('ProbeParent')), child=makeUser('deepChild',roles.get('ProbeChild')), owner=makeUser('deepOwner',roles.get('ProbeGrandchild')), peer=makeUser('deepPeer',roles.get('ProbeGrandchild')), other=makeUser('deepOther',null); insert new List<User>{parent,child,owner,peer,other};
  Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1].Id; List<PermissionSetAssignment> grants=new List<PermissionSetAssignment>(); for(User u:new List<User>{parent,child,owner,peer,other}) grants.add(new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=ps)); insert grants;
  System.runAs(owner){insert new PrivateProbe__c(Name='DeepOwned');}
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(parent){result.put('ancestor',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='DeepOwned']);}
  System.runAs(child){result.put('parent',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='DeepOwned']);}
  System.runAs(peer){result.put('peer',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='DeepOwned']);}
  System.runAs(other){result.put('unrelated',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='DeepOwned']);}
  System.assert(false,'OBSERVED|hierarchyDepth|'+JSON.serialize(result));
 }
 @IsTest static void controlledHierarchy() {
  Map<String,Id> roles=new Map<String,Id>(); for(UserRole r:[SELECT Id,DeveloperName FROM UserRole WHERE DeveloperName IN ('ProbeParent','ProbeGrandchild')]) roles.put(r.DeveloperName,r.Id);
  User ancestor=makeUser('detailAncestor',roles.get('ProbeParent')), owner=makeUser('detailDescendant',roles.get('ProbeGrandchild')), other=makeUser('detailUnrelated',null); insert new List<User>{ancestor,owner,other};
  Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1].Id; insert new List<PermissionSetAssignment>{new PermissionSetAssignment(AssigneeId=ancestor.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=owner.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=other.Id,PermissionSetId=ps)};
  Id parentId; System.runAs(owner){PrivateProbe__c p=new PrivateProbe__c(Name='Hierarchical parent');insert p;parentId=p.Id;insert new DetailProbe__c(Name='Hierarchical detail',Parent__c=p.Id);}
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(ancestor){result.put('ancestor',[SELECT COUNT() FROM DetailProbe__c WHERE Parent__c=:parentId]);}
  System.runAs(other){result.put('unrelated',[SELECT COUNT() FROM DetailProbe__c WHERE Parent__c=:parentId]);}
  System.assert(false,'OBSERVED|controlledHierarchy|'+JSON.serialize(result));
 }
 @IsTest static void controlledDetail() {
  User owner=makeUser('detailOwner',null), reader=makeUser('detailReader',null), other=makeUser('detailOther',null); insert new List<User>{owner,reader,other};
  Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1].Id; insert new List<PermissionSetAssignment>{new PermissionSetAssignment(AssigneeId=owner.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=reader.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=other.Id,PermissionSetId=ps)};
  Id parentId; System.runAs(owner){PrivateProbe__c p=new PrivateProbe__c(Name='Detail parent');insert p;parentId=p.Id;insert new DetailProbe__c(Name='Detail row',Parent__c=p.Id);}
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(reader){result.put('before',[SELECT COUNT() FROM DetailProbe__c WHERE Parent__c=:parentId]);result.put('locator',Database.getQueryLocator('SELECT Id FROM DetailProbe__c LIMIT 1').iterator().hasNext());try{DetailProbe__c d=[SELECT Id FROM DetailProbe__c WHERE Parent__c=:parentId LIMIT 1];result.put('singleton','found');}catch(QueryException e){result.put('singleton',e.getTypeName());}}
  Map<String,Id> groups=new Map<String,Id>(); for(Group g:[SELECT Id,DeveloperName FROM Group WHERE DeveloperName IN ('ProbeOrigin','ProbeTarget')]) groups.put(g.DeveloperName,g.Id);
  GroupMember origin=new GroupMember(GroupId=groups.get('ProbeOrigin'),UserOrGroupId=owner.Id), target=new GroupMember(GroupId=groups.get('ProbeTarget'),UserOrGroupId=reader.Id); insert new List<GroupMember>{origin,target};
  System.runAs(reader){result.put('after',[SELECT COUNT() FROM DetailProbe__c WHERE Parent__c=:parentId]);}
  System.runAs(other){result.put('other',[SELECT COUNT() FROM DetailProbe__c WHERE Parent__c=:parentId]);}
  delete target;
  System.runAs(owner){result.put('owner',[SELECT COUNT() FROM DetailProbe__c WHERE Parent__c=:parentId]);}
  User admin=[SELECT Id FROM User WHERE Id=:UserInfo.getUserId()]; System.runAs(admin){insert new PrivateProbe__Share(ParentId=parentId,UserOrGroupId=reader.Id,AccessLevel='Read',RowCause=Schema.PrivateProbe__Share.RowCause.Manual);}
  System.runAs(reader){result.put('manual',[SELECT COUNT() FROM DetailProbe__c WHERE Parent__c=:parentId]);}
  System.assert(false,'OBSERVED|controlledDetail|'+JSON.serialize(result));
 }
 @IsTest static void sharingRulesMultiple() {
  User ownerOne=makeUser('multiOwnerOne',null), ownerTwo=makeUser('multiOwnerTwo',null), readerOne=makeUser('multiReaderOne',null), readerTwo=makeUser('multiReaderTwo',null), other=makeUser('multiOther',null); insert new List<User>{ownerOne,ownerTwo,readerOne,readerTwo,other};
  Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1].Id; List<PermissionSetAssignment> grants=new List<PermissionSetAssignment>(); for(User u:new List<User>{ownerOne,ownerTwo,readerOne,readerTwo,other}) grants.add(new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=ps)); insert grants;
  Map<String,Id> groups=new Map<String,Id>(); for(Group g:[SELECT Id,DeveloperName FROM Group WHERE DeveloperName IN ('ProbeOrigin','ProbeTarget','ProbeOriginTwo','ProbeTargetTwo')]) groups.put(g.DeveloperName,g.Id);
  GroupMember targetTwo=new GroupMember(GroupId=groups.get('ProbeTargetTwo'),UserOrGroupId=readerTwo.Id); insert new List<GroupMember>{new GroupMember(GroupId=groups.get('ProbeOrigin'),UserOrGroupId=ownerOne.Id),new GroupMember(GroupId=groups.get('ProbeTarget'),UserOrGroupId=readerOne.Id),new GroupMember(GroupId=groups.get('ProbeOriginTwo'),UserOrGroupId=ownerTwo.Id),targetTwo};
  System.runAs(ownerOne){insert new PrivateProbe__c(Name='Multi First');} System.runAs(ownerTwo){insert new PrivateProbe__c(Name='Multi Second');}
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(readerOne){List<PrivateProbe__c> rows=[SELECT Name FROM PrivateProbe__c WHERE Name LIKE 'Multi %'];result.put('firstCount',rows.size());result.put('firstName',rows.isEmpty() ? 'none' : rows[0].Name);}
  System.runAs(readerTwo){List<PrivateProbe__c> rows=[SELECT Name FROM PrivateProbe__c WHERE Name LIKE 'Multi %'];result.put('secondCount',rows.size());result.put('secondName',rows.isEmpty() ? 'none' : rows[0].Name);}
  System.runAs(other){result.put('other',[SELECT COUNT() FROM PrivateProbe__c WHERE Name LIKE 'Multi %']);}
  delete targetTwo; System.runAs(readerTwo){result.put('removed',[SELECT COUNT() FROM PrivateProbe__c WHERE Name LIKE 'Multi %']);}
  System.assert(false,'OBSERVED|sharingRulesMultiple|'+JSON.serialize(result));
 }
 @IsTest static void sharingRule() {
  User owner=makeUser('ruleowner',null), recipient=makeUser('recipient',null), other=makeUser('ruleother',null); insert new List<User>{owner,recipient,other};
  Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1].Id; insert new List<PermissionSetAssignment>{new PermissionSetAssignment(AssigneeId=owner.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=recipient.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=other.Id,PermissionSetId=ps)};
  Map<String,Id> groups=new Map<String,Id>(); for(Group g:[SELECT Id,DeveloperName FROM Group WHERE DeveloperName IN ('ProbeOrigin','ProbeTarget')]) groups.put(g.DeveloperName,g.Id);
  insert new List<GroupMember>{new GroupMember(GroupId=groups.get('ProbeOrigin'),UserOrGroupId=owner.Id),new GroupMember(GroupId=groups.get('ProbeTarget'),UserOrGroupId=recipient.Id)};
  System.runAs(owner){ insert new PrivateProbe__c(Name='Shared'); }
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(recipient){result.put('recipient',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='Shared']);}
  System.runAs(other){result.put('other',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='Shared']);}
  System.assert(false,'OBSERVED|sharingRule|'+JSON.serialize(result));
 }
 @IsTest static void membershipChange() {
  User owner=makeUser('changeowner',null), recipient=makeUser('changereader',null); insert new List<User>{owner,recipient};
  Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1].Id; insert new List<PermissionSetAssignment>{new PermissionSetAssignment(AssigneeId=owner.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=recipient.Id,PermissionSetId=ps)};
  Map<String,Id> groups=new Map<String,Id>(); for(Group g:[SELECT Id,DeveloperName FROM Group WHERE DeveloperName IN ('ProbeOrigin','ProbeTarget')]) groups.put(g.DeveloperName,g.Id);
  GroupMember origin=new GroupMember(GroupId=groups.get('ProbeOrigin'),UserOrGroupId=owner.Id), target=new GroupMember(GroupId=groups.get('ProbeTarget'),UserOrGroupId=recipient.Id); insert new List<GroupMember>{origin,target};
  System.runAs(owner){insert new PrivateProbe__c(Name='Change');}
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(recipient){result.put('before',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='Change']);}
  delete origin;
  System.runAs(recipient){result.put('removed',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='Change']);}
  insert new GroupMember(GroupId=groups.get('ProbeOrigin'),UserOrGroupId=owner.Id);
  System.runAs(recipient){result.put('restored',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='Change']);}
  System.assert(false,'OBSERVED|membershipChange|'+JSON.serialize(result));
 }
 @IsTest static void ruleRoutes() {
  User owner=makeUser('routeowner',null), recipient=makeUser('routereader',null); insert new List<User>{owner,recipient};
  Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1].Id; insert new List<PermissionSetAssignment>{new PermissionSetAssignment(AssigneeId=owner.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=recipient.Id,PermissionSetId=ps)};
  Map<String,Id> groups=new Map<String,Id>(); for(Group g:[SELECT Id,DeveloperName FROM Group WHERE DeveloperName IN ('ProbeOrigin','ProbeTarget')]) groups.put(g.DeveloperName,g.Id);
  GroupMember target=new GroupMember(GroupId=groups.get('ProbeTarget'),UserOrGroupId=recipient.Id); insert new List<GroupMember>{new GroupMember(GroupId=groups.get('ProbeOrigin'),UserOrGroupId=owner.Id),target};
  Id recordId; System.runAs(owner){PrivateProbe__c row=new PrivateProbe__c(Name='Route');insert row;recordId=row.Id;}
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(recipient) {
   result.put('static',[SELECT Id FROM PrivateProbe__c WHERE Id=:recordId].size());
   result.put('dynamic',Database.query('SELECT Id FROM PrivateProbe__c WHERE Id = :recordId').size());
   result.put('access',[SELECT RecordId,HasReadAccess FROM UserRecordAccess WHERE UserId=:recipient.Id AND RecordId=:recordId].HasReadAccess);
  }
  delete target;
  System.runAs(recipient){result.put('recipientRemoved',[SELECT COUNT() FROM PrivateProbe__c WHERE Id=:recordId]);}
  System.assert(false,'OBSERVED|ruleRoutes|'+JSON.serialize(result));
 }
 @IsTest static void searchRelations() {
  User owner=makeUser('searchowner',null), recipient=makeUser('searchreader',null), other=makeUser('searchother',null); insert new List<User>{owner,recipient,other};
  Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1].Id; insert new List<PermissionSetAssignment>{new PermissionSetAssignment(AssigneeId=owner.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=recipient.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=other.Id,PermissionSetId=ps)};
  Map<String,Id> groups=new Map<String,Id>(); for(Group g:[SELECT Id,DeveloperName FROM Group WHERE DeveloperName IN ('ProbeOrigin','ProbeTarget')]) groups.put(g.DeveloperName,g.Id);
  insert new List<GroupMember>{new GroupMember(GroupId=groups.get('ProbeOrigin'),UserOrGroupId=owner.Id),new GroupMember(GroupId=groups.get('ProbeTarget'),UserOrGroupId=recipient.Id)};
  Id parentId,childId; System.runAs(owner){PrivateProbe__c p=new PrivateProbe__c(Name='Searchable');insert p;parentId=p.Id;ProbeChild__c c=new ProbeChild__c(Name='child',Parent__c=p.Id);insert c;childId=c.Id;}
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(recipient){ProbeChild__c c=[SELECT Parent__r.Name FROM ProbeChild__c WHERE Id=:childId];result.put('recipientParent',c.Parent__r==null ? 'null' : c.Parent__r.Name);Test.setFixedSearchResults(new Id[]{parentId});result.put('recipientSearch',Search.query('FIND \'Searchable\' IN ALL FIELDS RETURNING PrivateProbe__c(Id)')[0].size());}
  System.runAs(other){ProbeChild__c c=[SELECT Parent__r.Name FROM ProbeChild__c WHERE Id=:childId];result.put('otherParent',c.Parent__r==null ? 'null' : c.Parent__r.Name);Test.setFixedSearchResults(new Id[]{parentId});result.put('otherSearch',Search.query('FIND \'Searchable\' IN ALL FIELDS RETURNING PrivateProbe__c(Id)')[0].size());}
  System.assert(false,'OBSERVED|searchRelations|'+JSON.serialize(result));
 }
 @IsTest static void aggregateSelf() {
  User owner=makeUser('aggowner',null), recipient=makeUser('aggreader',null), other=makeUser('aggother',null); insert new List<User>{owner,recipient,other};
  Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1].Id; insert new List<PermissionSetAssignment>{new PermissionSetAssignment(AssigneeId=owner.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=recipient.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=other.Id,PermissionSetId=ps)};
  Map<String,Id> groups=new Map<String,Id>(); for(Group g:[SELECT Id,DeveloperName FROM Group WHERE DeveloperName IN ('ProbeOrigin','ProbeTarget')]) groups.put(g.DeveloperName,g.Id);
  insert new List<GroupMember>{new GroupMember(GroupId=groups.get('ProbeOrigin'),UserOrGroupId=owner.Id),new GroupMember(GroupId=groups.get('ProbeTarget'),UserOrGroupId=recipient.Id)};
  Id parentId,childId;System.runAs(owner){PrivateProbe__c p=new PrivateProbe__c(Name='Parent');insert p;parentId=p.Id;PrivateProbe__c c=new PrivateProbe__c(Name='Child',Parent__c=p.Id);insert c;childId=c.Id;}
  Set<Id> ids=new Set<Id>{parentId,childId};Map<String,Object> result=new Map<String,Object>();
  System.runAs(recipient){List<AggregateResult> a=[SELECT Name,COUNT(Id) n FROM PrivateProbe__c WHERE Id IN :ids GROUP BY Name];List<PrivateProbe__c> c=[SELECT Parent__r.Name FROM PrivateProbe__c WHERE Id=:childId];result.put('recipientGroups',a.size());result.put('recipientParent',c[0].Parent__r.Name);}
  System.runAs(other){List<AggregateResult> a=[SELECT Name,COUNT(Id) n FROM PrivateProbe__c WHERE Id IN :ids GROUP BY Name];List<PrivateProbe__c> c=[SELECT Parent__r.Name FROM PrivateProbe__c WHERE Id=:childId];result.put('otherGroups',a.size());result.put('otherRows',c.size());}
  System.assert(false,'OBSERVED|aggregateSelf|'+JSON.serialize(result));
 }
}
