Account account = new Account(Name = 'Snippet Probe');
insert account;
Test.setFixedSearchResults(new List<Id>{account.Id});
Search.SearchResults results = Search.find('FIND {Snippet*} IN ALL FIELDS RETURNING Account(Id, Name)');
List<Search.SearchResult> accounts = results.get('Account');
System.assertEquals(1, accounts.size(), 'fixed result is returned');
String snippet = accounts[0].getSnippet();
System.debug(LoggingLevel.ERROR, 'SEARCH_FIXED_SNIPPET|' + String.valueOf(snippet));
