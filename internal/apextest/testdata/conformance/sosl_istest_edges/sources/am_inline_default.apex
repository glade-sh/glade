System.runAs(usr) { List<List<SObject>> r = [FIND 'amtest' RETURNING Account(Id, Name)]; obs = SoslObs__A__.sizes(r); }
