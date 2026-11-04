for (String n : new List<String>{'Case251Picklist59','Case251Picklist67'}) { PageReference p = new PageReference('/apex/'+n); System.debug('CASE251PAGE:'+n+':'+p.getContent().toString()); }
