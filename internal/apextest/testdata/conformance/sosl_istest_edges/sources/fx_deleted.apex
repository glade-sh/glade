Id gone = a3.Id;
delete a3;
Test.setFixedSearchResults(new List<Id>{ a1.Id, gone });
obs = SoslObs__A__.fmt(Search.query('FIND {nomatch} RETURNING Account(Name)'));
