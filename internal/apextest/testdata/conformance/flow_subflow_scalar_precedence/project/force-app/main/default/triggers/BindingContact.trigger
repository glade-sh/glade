trigger BindingContact on Contact (after insert, after update) {
 List<Contact> updates=new List<Contact>();
 for(Contact item:Trigger.new) {
  if(item.LastName.startsWith('Binding ') && (Trigger.isInsert || Trigger.oldMap.get(item.Id).LastName!=item.LastName)) {
   List<String> parts=item.LastName.split(' ');
   updates.add(new Contact(Id=item.Id,Description=BindingProbe.invoke(parts[1],parts[2],item.AccountId)));
  }
 }
 if(!updates.isEmpty()) update updates;
}
