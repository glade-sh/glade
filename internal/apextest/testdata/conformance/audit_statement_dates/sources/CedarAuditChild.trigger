trigger CedarAuditChild on Account (after insert) {
    List<Contact> children=new List<Contact>();
    for(Account parent:Trigger.new) {
        if(parent.Name=='Cedar trigger parent') children.add(new Contact(LastName='Cedar child',AccountId=parent.Id));
    }
    if(!children.isEmpty()) insert children;
}
