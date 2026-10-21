@IsTest private class TaskFlagRegression {
 @IsTest static void storedHighFlagUsesPriorityMetadata(){
  Task item=new Task(Subject='Flag regression',Priority='High');insert item;
  Task selected=[SELECT Priority,IsHighPriority FROM Task WHERE Id=:item.Id];
  System.assertEquals(true,selected.IsHighPriority);
  System.assertEquals(true,selected.get('IsHighPriority'));
  System.assertEquals(true,selected.get(Task.IsHighPriority));
 }
 @IsTest static void highToLowRecomputesStoredFlag(){
  Task item=new Task(Subject='Flag update',Priority='High');insert item;
  System.assertEquals(true,[SELECT IsHighPriority FROM Task WHERE Id=:item.Id].IsHighPriority);
  item.Priority='Low';update item;
  System.assertEquals(false,[SELECT IsHighPriority FROM Task WHERE Id=:item.Id].IsHighPriority);
 }
 @IsTest static void ordinaryPriorityControls(){
  for(String priority:new List<String>{'Normal','Low'}){
   Task item=new Task(Subject='Flag control',Priority=priority);insert item;
   System.assertEquals(false,[SELECT IsHighPriority FROM Task WHERE Id=:item.Id].IsHighPriority);
  }
 }
 @IsTest static void omittedPriorityUsesConfiguredDefault(){
  Task item=new Task(Subject='Default control');insert item;
  Task selected=[SELECT Priority,IsHighPriority FROM Task WHERE Id=:item.Id];
  Boolean expected=[SELECT IsHighPriority FROM TaskPriority WHERE ApiName=:selected.Priority LIMIT 1].IsHighPriority;
  System.assertEquals(expected,selected.IsHighPriority);
 }
 @IsTest static void ordinaryLeadCheckboxPersists(){
  Lead parent=new Lead(LastName='Flag',Company='Probe',HasOptedOutOfEmail=true);insert parent;
  System.assertEquals(true,[SELECT HasOptedOutOfEmail FROM Lead WHERE Id=:parent.Id].HasOptedOutOfEmail);
  parent.HasOptedOutOfEmail=false;update parent;
  System.assertEquals(false,[SELECT HasOptedOutOfEmail FROM Lead WHERE Id=:parent.Id].HasOptedOutOfEmail);
 }
 @IsTest static void loadedCheckboxEqualityIsPreserved(){
  SObject left=LyricTile__c.SObjectType.newSObject(null,true);
  SObject right=left.clone(true,true,true,true);
  left.put('VerseSwitch__c',false);
  System.assertEquals(right,left);
 }
}
