Test.setFixedSearchResults(new List<Id>());
List<List<SObject>> r = Search.query('FIND {nomatch} RETURNING Account(Name)');
obs = r.size() + '|' + SoslObs__A__.sizes(r);
