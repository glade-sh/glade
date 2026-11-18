List<String> rows = new List<String>();
try {
 Account a = (Account) JSON.deserialize('{"Name":"X"}', Account.class);
 insert a;
 rows.add('acct=ok');
} catch (Exception e) { rows.add('acct=' + e.getTypeName() + '|' + e.getMessage()); }
try {
 Contact c = (Contact) JSON.deserialize('{"FirstName":"F"}', Contact.class);
 insert c;
 rows.add('contact=ok');
} catch (Exception e) { rows.add('contact=' + e.getTypeName() + '|' + e.getMessage()); }
try { JSON.deserialize('{"NumberOfEmployees":"abc"}', Account.class); rows.add('mismatch=noerr'); } catch (Exception e) { rows.add('mismatch=' + e.getTypeName() + '|' + e.getMessage()); }
try { JSON.deserializeStrict('{"Name":"X","NoSuchField__c":"y"}', Account.class); rows.add('strict=noerr'); } catch (Exception e) { rows.add('strict=' + e.getTypeName() + '|' + e.getMessage()); }
for (String r : rows) { System.debug('OBS|' + r); }
