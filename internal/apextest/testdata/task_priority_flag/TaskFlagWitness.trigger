trigger TaskFlagWitness on Task (before insert,after insert,before update,after update){
 String phase=Trigger.isBefore?'before':'after';
 phase+=Trigger.isInsert?'Insert':'Update';
 TaskFlagCapture.recordPhase(phase,Trigger.new,Trigger.isUpdate?Trigger.old:null);
}
