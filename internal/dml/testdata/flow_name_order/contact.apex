List<String> rows = new List<String>();
List<Contact> contacts = new List<Contact>{new Contact(FirstName='Ada', LastName='Probe'), new Contact(FirstName='Other', LastName='Probe'), new Contact(FirstName='Reject', LastName='Probe'), new Contact(FirstName='NullProbe', LastName='Probe'), new Contact(FirstName='NonNullProbe', LastName='Probe')};
for (Contact c : contacts) { Savepoint sp=Database.setSavepoint(); Database.SaveResult r = Database.insert(c, false); rows.add(String.valueOf(r.isSuccess()) + '|' + (r.isSuccess() ? '' : r.getErrors()[0].getMessage())); Database.rollback(sp); }
for (String row : rows) System.debug('NAME_ORDER|contact|' + row);
