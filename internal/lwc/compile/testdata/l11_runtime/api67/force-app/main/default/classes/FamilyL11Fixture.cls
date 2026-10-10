public with sharing class FamilyL11Fixture {
    // setup/cleanup are called once by the serialized native CLI, never by a
    // parallel page. Only this family's exact names and validated children may
    // be repaired or removed. Unexpected content fails before any deletion.
    private static final List<String> CHILD_NAMES=new List<String>{'L11 A','L11 B','L11 C'};
    private static void checkApi(String sourceApi) {
        if (!new Set<String>{'59.0','60.0','61.0','62.0','63.0','64.0','65.0','66.0','67.0'}.contains(sourceApi)) {
            throw new AuraHandledException('L11_SOURCE_API_INVALID');
        }
    }
    private static List<FamilyL11Parent__c> ownedParents() {
        return [SELECT Id,Name FROM FamilyL11Parent__c WHERE Name IN ('L11 Parent','L11 Empty') ORDER BY CreatedDate,Id];
    }
    private static List<FamilyL11Child__c> checkedChildren(List<FamilyL11Parent__c> parents) {
        Map<Id,FamilyL11Parent__c> byId=new Map<Id,FamilyL11Parent__c>(parents);
        Set<Id> ids=byId.keySet();
        List<FamilyL11Child__c> children=[SELECT Id,Name,Sort__c,Parent__c FROM FamilyL11Child__c WHERE Parent__c IN :ids ORDER BY CreatedDate,Id];
        for(FamilyL11Child__c child:children) {
            if(byId.get(child.Parent__c).Name!='L11 Parent' || child.Sort__c==null ||
               child.Sort__c<1 || child.Sort__c>3 || child.Sort__c!=child.Sort__c.intValue() ||
               child.Name!=CHILD_NAMES[child.Sort__c.intValue()-1]) {
                throw new AuraHandledException('L11_FIXTURE_CONTENT_CHANGED');
            }
        }
        return children;
    }
    public static void setup(String sourceApi) {
        checkApi(sourceApi);
        List<FamilyL11Parent__c> parents=ownedParents();
        List<FamilyL11Child__c> children=checkedChildren(parents);
        FamilyL11Parent__c parent,empty;
        List<FamilyL11Parent__c> extraParents=new List<FamilyL11Parent__c>();
        for(FamilyL11Parent__c row:parents) {
            if(row.Name=='L11 Parent' && parent==null) parent=row;
            else if(row.Name=='L11 Empty' && empty==null) empty=row;
            else extraParents.add(row);
        }
        if(parent==null) {parent=new FamilyL11Parent__c(Name='L11 Parent');insert parent;}
        if(empty==null) {empty=new FamilyL11Parent__c(Name='L11 Empty');insert empty;}
        Map<Integer,FamilyL11Child__c> kept=new Map<Integer,FamilyL11Child__c>();
        List<FamilyL11Child__c> extraChildren=new List<FamilyL11Child__c>();
        for(FamilyL11Child__c child:children) {
            Integer position=child.Sort__c.intValue();
            if(child.Parent__c==parent.Id && !kept.containsKey(position)) kept.put(position,child);
            else extraChildren.add(child);
        }
        if(!extraChildren.isEmpty()) delete extraChildren;
        if(!extraParents.isEmpty()) delete extraParents;
        List<FamilyL11Child__c> missing=new List<FamilyL11Child__c>();
        for(Integer i=1;i<=3;i++) if(!kept.containsKey(i)) {
            missing.add(new FamilyL11Child__c(Name=CHILD_NAMES[i-1],Sort__c=i,Parent__c=parent.Id));
        }
        if(!missing.isEmpty()) insert missing;
        prepare(sourceApi);
    }
    public static void cleanup(String sourceApi) {
        checkApi(sourceApi);
        List<FamilyL11Parent__c> parents=ownedParents();
        List<FamilyL11Child__c> children=checkedChildren(parents);
        if(!children.isEmpty()) delete children;
        if(!parents.isEmpty()) delete parents;
    }
    @AuraEnabled
    public static Map<String,String> prepare(String sourceApi) {
        // Read-only: multiple browser pages can safely validate the one fixture.
        checkApi(sourceApi);
        String tag='L11 Parent';
        List<FamilyL11Parent__c> parents=[SELECT Id FROM FamilyL11Parent__c WHERE Name=:tag];
        if (parents.size()!=1) throw new AuraHandledException('L11_FIXTURE_NOT_READY');
        FamilyL11Parent__c parent=parents[0];
        String emptyTag='L11 Empty';
        List<FamilyL11Parent__c> empties=[SELECT Id FROM FamilyL11Parent__c WHERE Name=:emptyTag];
        if (empties.size()!=1) throw new AuraHandledException('L11_FIXTURE_NOT_READY');
        FamilyL11Parent__c empty=empties[0];
        List<FamilyL11Child__c> children=[SELECT Id,Name,Sort__c FROM FamilyL11Child__c WHERE Parent__c=:parent.Id ORDER BY Sort__c];
        if(children.size()!=3) throw new AuraHandledException('L11_FIXTURE_COUNT_CHANGED');
        if([SELECT count() FROM FamilyL11Child__c WHERE Parent__c=:empty.Id]!=0) throw new AuraHandledException('L11_EMPTY_FIXTURE_CHANGED');
        Map<String,String> result=new Map<String,String>{'parent'=>parent.Id,'empty'=>empty.Id};
        for(Integer i=0;i<3;i++) {
            if(children[i].Sort__c!=i+1 || children[i].Name!=CHILD_NAMES[i]) throw new AuraHandledException('L11_FIXTURE_CONTENT_CHANGED');
            result.put('child'+String.valueOf(i+1),children[i].Id);
        }
        return result;
    }
}
