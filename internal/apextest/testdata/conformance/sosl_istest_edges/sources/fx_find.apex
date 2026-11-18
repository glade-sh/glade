Test.setFixedSearchResults(new List<Id>{ a1.Id, a1.Id });
obs = '' + Search.find('FIND {nomatch} RETURNING Account(Name)').get('Account').size();
