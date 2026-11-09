public class StdCheckboxProbe {
 static final List<String> PAIRS = new List<String>{'Contact.IsEmailBounced','Lead.IsConverted','Lead.IsUnreadByOwner','Opportunity.IsPrivate','Case.IsEscalated','Task.IsReminderSet','Event.IsPrivate','Product2.IsActive','Campaign.IsActive','Pricebook2.IsActive','User.IsActive','Account.IsDeleted'};
 public static Map<String,Object> observe(String shape) {
  Map<String,Object> r = new Map<String,Object>();
  if (shape == 'schemaNew' || shape == 'schemaNewToken') {
   for (String p : PAIRS) {
    List<String> parts = p.split('\\.');
    SObject s = Schema.getGlobalDescribe().get(parts[0]).newSObject();
    if (shape == 'schemaNew') { r.put(p, s.get(parts[1])); }
    else { r.put(p, s.get(s.getSObjectType().getDescribe().fields.getMap().get(parts[1]))); }
   }
  } else if (shape == 'ctor') {
   List<SObject> rows = new List<SObject>{new Contact(), new Lead(), new Opportunity(), new Case(), new Task(), new Event(), new Product2(), new Campaign(), new Pricebook2(), new User(), new Account()};
   for (String p : PAIRS) {
    String o = p.substringBefore('.');
    for (SObject s : rows) { if (String.valueOf(s.getSObjectType()) == o) { r.put(p, s.get(p.substringAfter('.'))); } }
   }
  } else if (shape == 'dot') {
   Contact c = new Contact(); Lead l = new Lead(); Opportunity o = new Opportunity(); Case cs = new Case(); Task t = new Task();
   Product2 pr = new Product2(); Campaign cp = new Campaign(); User u = new User(); Account a = new Account();
   r.put('Contact.IsEmailBounced', c.IsEmailBounced); r.put('Lead.IsConverted', l.IsConverted); r.put('Opportunity.IsPrivate', o.IsPrivate);
   r.put('Case.IsEscalated', cs.IsEscalated); r.put('Task.IsReminderSet', t.IsReminderSet); r.put('Product2.IsActive', pr.IsActive);
   r.put('Campaign.IsActive', cp.IsActive); r.put('User.IsActive', u.IsActive);
   SObject n = Schema.getGlobalDescribe().get('Product2').newSObject();
   r.put('schemaNewCompare', n.get('IsActive') == false);
   r.put('schemaNewIsSet', n.isSet('IsActive'));
   r.put('schemaNewPopulated', n.getPopulatedFieldsAsMap().keySet().size());
  } else if (shape == 'inserted') {
   Product2 p = new Product2(Name = 'p'); insert p;
   Contact c = new Contact(LastName = 'z'); insert c;
   Campaign cp = new Campaign(Name = 'c'); insert cp;
   r.put('Product2.IsActive', p.get('IsActive')); r.put('Contact.IsEmailBounced', c.get('IsEmailBounced')); r.put('Campaign.IsActive', cp.get('IsActive'));
   r.put('Product2.dot', p.IsActive);
   r.put('Product2.queried', ((Product2)[SELECT IsActive FROM Product2 WHERE Id = :p.Id]).get('IsActive'));
  } else if (shape == 'clone') {
   Product2 p = new Product2(Name = 'p'); Product2 q = p.clone(); Product2 s = (Product2)Schema.getGlobalDescribe().get('Product2').newSObject().clone();
   r.put('Product2.cloneNew', q.get('IsActive')); r.put('Product2.cloneSchemaNew', s.get('IsActive'));
   r.put('Product2.cloneDot', q.IsActive);
  } else if (shape == 'put') {
   SObject n = Schema.getGlobalDescribe().get('Product2').newSObject();
   r.put('putReturn', n.put('IsActive', false)); r.put('afterPut', n.get('IsActive'));
   r.put('putTrue', n.put('IsActive', true)); r.put('afterPutTrue', n.get('IsActive'));
  }
  return r;
 }
}
