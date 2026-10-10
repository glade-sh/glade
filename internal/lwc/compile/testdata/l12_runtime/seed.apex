// Coordinator only, after owned runtime metadata deploy. No capture or deletion.
// Upsert only these five owned external keys; never query or modify other objects.
FamilyL12Record__c parent = new FamilyL12Record__c(
    Key__c='L12_PARENT', Name='Owned parent', Label__c='parent',
    Amount__c=100, Active__c=true, Day__c=null, Moment__c=null, Parent__c=null);
upsert parent Key__c;
List<FamilyL12Record__c> owned = new List<FamilyL12Record__c>{
    new FamilyL12Record__c(Key__c='L12_A', Name='Owned alpha', Label__c='alpha',
        Amount__c=12.50, Active__c=true, Day__c=Date.newInstance(2024,2,29),
        Moment__c=Datetime.newInstanceGmt(2024,2,29,12,30,0), Parent__c=parent.Id),
    new FamilyL12Record__c(Key__c='L12_B', Name='Owned beta', Label__c=null,
        Amount__c=null, Active__c=false, Day__c=null, Moment__c=null, Parent__c=null),
    new FamilyL12Record__c(Key__c='L12_C', Name='Owned é水😀', Label__c='é水😀',
        Amount__c=0, Active__c=false, Day__c=Date.newInstance(2024,1,1),
        Moment__c=Datetime.newInstanceGmt(2024,1,1,0,0,0), Parent__c=parent.Id),
    new FamilyL12Record__c(Key__c='L12_D', Name='Owned delta', Label__c='',
        Amount__c=-2, Active__c=false, Day__c=null, Moment__c=null, Parent__c=null)
};
upsert owned Key__c;
System.debug(LoggingLevel.ERROR, 'L12 fixture seed completed: five owned keys');
