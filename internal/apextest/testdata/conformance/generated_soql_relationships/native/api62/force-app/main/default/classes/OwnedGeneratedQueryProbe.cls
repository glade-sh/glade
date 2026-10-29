@IsTest
public class OwnedGeneratedQueryProbe {
    @IsTest static void observe() {
        Owned_Query_Person__c person = new Owned_Query_Person__c(
            Name='Probe Person',
            External_Key__c='CODE-123'
        );
        insert person;

        Account accountRecord = new Account(
            Name='Probe Account',
            Related_Person__c=person.Id
        );
        insert accountRecord;

        Case caseRecord = new Case(
            Subject='Probe Case',
            Status='New',
            Origin='Web',
            AccountId=accountRecord.Id
        );
        insert caseRecord;
        Owned_Query_Conversation__c conversationRecord = new Owned_Query_Conversation__c(
            Name='Probe Conversation',
            Related_Account__c=accountRecord.Id,
            Related_Case__c=caseRecord.Id
        );
        insert conversationRecord;

        List<Account> byParentName = [
            SELECT Id
            FROM Account
            WHERE Related_Person__r.Name = 'Probe Person'
        ];
        List<Account> byParentCustomField = [
            SELECT Id
            FROM Account
            WHERE Related_Person__r.External_Key__c = 'CODE-123'
        ];
        Set<String> actorKeys = new Set<String>{ 'CODE-123' };
        String accountName = 'Probe Account';
        String generatedFilter = 'SELECT Id FROM Account WHERE (Related_Person__r.External_Key__c IN :actorKeys AND Name = :accountName) LIMIT 1';
        List<SObject> generatedRelatedFilter = Database.query(generatedFilter);
        Id caseId = caseRecord.Id;
        String generatedTwoHopFilter = 'SELECT Id FROM Owned_Query_Conversation__c WHERE (Related_Account__r.Related_Person__r.External_Key__c IN :actorKeys AND Related_Case__c = :caseId) LIMIT 1';
        List<SObject> generatedTwoHopRelatedFilter = Database.query(generatedTwoHopFilter);
        String accountSafeName = [
            SELECT Related_Person__r.Name
            FROM Account
            WHERE Id = :accountRecord.Id
            LIMIT 1
        ]
            ?.Related_Person__r
            ?.Name;
        String caseSafeKey = [
            SELECT Account.Related_Person__r.External_Key__c
            FROM Case
            WHERE Id = :caseRecord.Id
            LIMIT 1
        ]
            ?.Account
            ?.Related_Person__r
            ?.External_Key__c;
        User currentUser = [
            SELECT Account.Related_Person__r.External_Key__c
            FROM User
            WHERE Id = :UserInfo.getUserId()
            LIMIT 1
        ];
        String userRelatedKey = currentUser?.Account?.Related_Person__r?.External_Key__c;
        List<Owned_Query_Setting__mdt> settings = [
            SELECT Record_Type_Name__c
            FROM Owned_Query_Setting__mdt
            WHERE DeveloperName = 'Native_Probe'
        ];
        Owned_Query_Setting__mdt setting;
        if (settings.size() > 0) {
            setting = settings[0];
        }
        String recordTypeName = setting?.Record_Type_Name__c;
        String recordTypeId = [
            SELECT Id
            FROM RecordType
            WHERE developerName = :setting?.Record_Type_Name__c
        ]?.Id;

        Flow.Interview.Owned_Generated_Query_Get_Records interview =
            new Flow.Interview.Owned_Generated_Query_Get_Records(
                new Map<String, Object>{ 'AccountId' => accountRecord.Id, 'RecordId' => caseRecord.Id }
            );
        interview.start();
        String flowRelatedKey = (String) interview.getVariableValue('RelatedKey');
        String twoHopFlowRelatedKey = (String) interview.getVariableValue('TwoHopRelatedKey');

        Map<String, Object> observed = new Map<String, Object>{
            'accountFilterParentNameCount' => byParentName.size(),
            'accountFilterParentCustomFieldCount' => byParentCustomField.size(),
            'dynamicRelationshipFilterCount' => generatedRelatedFilter.size(),
            'twoHopDynamicRelationshipFilterCount' => generatedTwoHopRelatedFilter.size(),
            'accountProjectedSafeName' => accountSafeName,
            'caseProjectedSafeKey' => caseSafeKey,
            'flowGetRecordsRelatedKey' => flowRelatedKey,
            'twoHopFlowGetRecordsRelatedKey' => twoHopFlowRelatedKey,
            'recordTypeBindName' => recordTypeName,
            'recordTypeQueryResultSafeNavigation' => recordTypeId != null,
            'userNestedRelatedKey' => userRelatedKey
        };
        System.assertEquals('capture', 'OBSERVED|generatedSOQLRelationship|' + JSON.serialize(observed));
    }
}
