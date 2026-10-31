Map<String,Object> obs = new Map<String,Object>();
List<Account> records = new List<Account>{new Account(Name='first'),new Account(Name='second'),new Account()};
try { Database.insert(records,true); } catch (DmlException e) { obs.put('default',e.getMessage()); }
try { Database.insert(records,true,AccessLevel.USER_MODE); } catch (DmlException e) { obs.put('user',e.getMessage()); }
System.debug('OBS|'+JSON.serialize(obs));
