Map<String,Object> o=new Map<String,Object>();
o.put('J01',new List<Integer>{System.hashCode((Account)JSON.deserialize('{}',Account.class)),System.hashCode((Contact)JSON.deserialize('{}',Contact.class))});
o.put('J02',new List<Integer>{System.hashCode((Account)JSON.deserialize('{"Name":"Synthetic"}',Account.class)),System.hashCode((Account)JSON.deserialize('{"Name":null}',Account.class)),System.hashCode((Contact)JSON.deserialize('{"LastName":"Other"}',Contact.class))});
o.put('J03',new List<Integer>{System.hashCode((Opportunity)JSON.deserializeStrict('{"Name":"Synthetic"}',Opportunity.class)),System.hashCode((Lead)JSON.deserializeStrict('{"LastName":null}',Lead.class))});
o.put('J05',new List<Integer>{System.hashCode((Case)JSON.deserialize('{}',Case.class)),System.hashCode((Account)JSON.deserialize('{"Name":"Synthetic","Industry":"Other"}',Account.class)),System.hashCode((SObject)JSON.deserialize('{"attributes":{"type":"Account"},"Name":"Synthetic"}',SObject.class))});
try { JSON.deserialize('[]',Account.class); o.put('J04','accepted'); } catch(Exception e) {o.put('J04',e.getTypeName());}
System.debug('OBS|' + JSON.serialize(o));
