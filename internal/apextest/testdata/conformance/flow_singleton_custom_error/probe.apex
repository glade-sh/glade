Database.SaveResult r = Database.insert(new Account(Name='control'), false);
System.debug('OBS|' + (r.isSuccess() ? 'SUCCESS' : String.valueOf(r.getErrors()[0].getStatusCode()) + '|' + r.getErrors()[0].getMessage()));
