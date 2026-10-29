@IsTest
public class OwnedRelationshipProbe62 {
    @IsTest static void observe() {
        Owned_Relationship_Parent__c parent = new Owned_Relationship_Parent__c(Name='Owned Parent API 62');
        insert parent;

        Owned_Relationship_Child__c child = new Owned_Relationship_Child__c(Name='Owned Child API 62', Related_Parent__c=parent.Id);
        insert child;

        Owned_Relationship_Child__c selected = [
            SELECT Related_Parent__r.Name
            FROM Owned_Relationship_Child__c
            WHERE Id = :child.Id
        ];

        String queryText = 'SELECT Id FROM Owned_Relationship_Child__c WHERE Related_Parent__r.Name = \'' + String.escapeSingleQuotes(parent.Name) + '\'';
        List<SObject> generated = Database.query(queryText);

        Account accountRecord = new Account(Name='Owned Account API 62', Owned_Parent__c=parent.Id);
        insert accountRecord;
        String accountQueryText = 'SELECT Id FROM Account WHERE Owned_Parent__r.Name = \'' + String.escapeSingleQuotes(parent.Name) + '\'';
        List<SObject> accountGenerated = Database.query(accountQueryText);

        Owned_Relationship_Child__c holder = new Owned_Relationship_Child__c(Name=parent.Name);
        List<Owned_Relationship_Child__c> bound = [
            SELECT Id, Related_Parent__r.Name
            FROM Owned_Relationship_Child__c
            WHERE Related_Parent__r.Name = :holder?.Name
        ];
        Flow.Interview.Owned_Relationship_Get_Records interview = new Flow.Interview.Owned_Relationship_Get_Records(
            new Map<String, Object>{'ChildId' => child.Id}
        );
        interview.start();
        String flowName = (String)interview.getVariableValue('RelatedName');
        Map<String, Object> observed = new Map<String, Object>{
            'flowGetRecordsRelatedField' => flowName,
            'relationshipSoqlSelection' => selected.Related_Parent__r.Name,
            'generatedSoqlRelationshipFilterCount' => generated.size(),
            'generatedSoqlAccountRelationshipFilterCount' => accountGenerated.size(),
            'relationshipSoqlSafeNavigationBindCount' => bound.size(),
            'relationshipSoqlSafeNavigationBindName' => bound[0].Related_Parent__r.Name
        };
        System.assertEquals('capture', 'OBSERVED|flowRelatedFields|' + JSON.serialize(observed));
    }
}
