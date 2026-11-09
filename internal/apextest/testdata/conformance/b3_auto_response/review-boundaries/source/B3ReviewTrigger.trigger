trigger B3ReviewTrigger on Case (before insert) {
 for(Case c:Trigger.new) {
  if(c.Subject=='B3Populate') c.ContactId=[SELECT Id FROM Contact WHERE LastName='B3Valid' LIMIT 1].Id;
  if(c.Subject=='B3Clear') c.ContactId=null;
 }
}
