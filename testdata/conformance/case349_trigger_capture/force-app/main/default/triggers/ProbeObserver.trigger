trigger ProbeObserver on Account (before insert, before update) {
 ProbeControl.hits++;
 for(Account a : Trigger.new) {
  if(Trigger.isUpdate && ProbeControl.mode=='same-name') a.Name=a.Name;
  if(Trigger.isUpdate && ProbeControl.mode=='same-type') a.RecordTypeId=a.RecordTypeId;
  if(Trigger.isUpdate && ProbeControl.mode=='changed-name') a.Name='Changed Synthetic';
 }
}
