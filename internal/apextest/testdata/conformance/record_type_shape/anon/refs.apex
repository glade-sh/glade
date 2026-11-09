Map<String,Schema.SObjectType> g = Schema.getGlobalDescribe();
Map<String,Object> m = new Map<String,Object>();
for (String p : new List<String>{'CollaborationGroup.NetworkId','Contact.AccountId','Case.ContactId','Opportunity.AccountId','Attachment.ParentId','Note.ParentId','ContentDocumentLink.LinkedEntityId','FeedItem.ParentId','EntitySubscription.ParentId','Task.WhoId','Event.WhatId','User.ContactId','Opportunity.SyncedQuoteId','Topic.NetworkId','Case.OwnerId','Asset.AccountId','CampaignMember.LeadOrContactId'}) {
  String o = p.substringBefore('.'); String f = p.substringAfter('.');
  if (!g.containsKey(o)) { m.put(p, 'noObject'); continue; }
  Schema.SObjectField t = g.get(o).getDescribe().fields.getMap().get(f);
  if (t == null) { m.put(p, 'noField'); continue; }
  List<String> names = new List<String>(); Integer inG = 0;
  for (Schema.SObjectType r : t.getDescribe().getReferenceTo()) { String n = r.getDescribe().getName(); names.add(n); if (g.containsKey(n)) inG++; }
  m.put(p, names.size() <= 12 ? String.join(names, ',') : (names.size() + ' targets, inGlobal ' + inG));
}
System.debug('OBS|' + JSON.serialize(m));
