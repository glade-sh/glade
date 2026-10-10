public with sharing class FamilyL11Fixture {
    // Fixture data only; leave pre-existing or unowned records unchanged.
    @AuraEnabled
    public static Map<String,String> prepare(String sourceApi) {
        if (!new Set<String>{'59.0','60.0','61.0','62.0','63.0','64.0','65.0','66.0','67.0'}.contains(sourceApi)) {
            throw new AuraHandledException('L11_SOURCE_API_INVALID');
        }
        List<String> names=new List<String>{'L11 A','L11 B','L11 C'};
        String tag='L11 Parent';
        List<FamilyL11Parent__c> parents=[SELECT Id FROM FamilyL11Parent__c WHERE Name=:tag];
        if (parents.size()>1) throw new AuraHandledException('L11_DUPLICATE_FIXTURE');
        FamilyL11Parent__c parent=parents.isEmpty()?new FamilyL11Parent__c(Name=tag):parents[0];
        if (parents.isEmpty()) insert parent;
        String emptyTag='L11 Empty';
        List<FamilyL11Parent__c> empties=[SELECT Id FROM FamilyL11Parent__c WHERE Name=:emptyTag];
        if (empties.size()>1) throw new AuraHandledException('L11_DUPLICATE_FIXTURE');
        FamilyL11Parent__c empty=empties.isEmpty()?new FamilyL11Parent__c(Name=emptyTag):empties[0];
        if (empties.isEmpty()) insert empty;
        List<FamilyL11Child__c> children=[SELECT Id,Name,Sort__c FROM FamilyL11Child__c WHERE Parent__c=:parent.Id ORDER BY Sort__c];
        if (children.isEmpty()) {
            children=new List<FamilyL11Child__c>();
            for(Integer i=1;i<=3;i++) children.add(new FamilyL11Child__c(Name=names[i-1],Sort__c=i,Parent__c=parent.Id));
            insert children;
        }
        if(children.size()!=3) throw new AuraHandledException('L11_FIXTURE_COUNT_CHANGED');
        if([SELECT count() FROM FamilyL11Child__c WHERE Parent__c=:empty.Id]!=0) throw new AuraHandledException('L11_EMPTY_FIXTURE_CHANGED');
        Map<String,String> result=new Map<String,String>{'parent'=>parent.Id,'empty'=>empty.Id};
        for(Integer i=0;i<3;i++) {
            if(children[i].Sort__c!=i+1 || children[i].Name!=names[i]) throw new AuraHandledException('L11_FIXTURE_CONTENT_CHANGED');
            result.put('child'+String.valueOf(i+1),children[i].Id);
        }
        return result;
    }
}
