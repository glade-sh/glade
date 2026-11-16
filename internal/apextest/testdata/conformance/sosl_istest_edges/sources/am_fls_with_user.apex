System.runAs(usr2) { List<List<SObject>> r = Search.query('FIND {amtest} RETURNING Account(Id, Phone) WITH USER_MODE'); obs = r[0].size() + '|' + ((Account) r[0][0]).Phone; }
