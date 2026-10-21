Account account = new Account(Name = 'Snippet Probe');
insert account;
String observation = 'FIXED_RESULTS_ACCEPTED';
try {
    Test.setFixedSearchResults(new List<Id>{account.Id});
} catch (Exception ex) {
    observation = ex.getTypeName() + '|' + ex.getMessage();
}
System.debug(LoggingLevel.ERROR, 'SEARCH_FIXED_SNIPPET|' + observation);
if (observation == 'FIXED_RESULTS_ACCEPTED') {
    Search.SearchResults results = Search.find('FIND {Snippet*} IN ALL FIELDS RETURNING Account(Id, Name)');
    List<Search.SearchResult> accounts = results.get('Account');
    System.debug(LoggingLevel.ERROR, 'SEARCH_FIXED_SNIPPET|count=' + accounts.size());
    if (!accounts.isEmpty()) {
        System.debug(LoggingLevel.ERROR, 'SEARCH_FIXED_SNIPPET|value=' + String.valueOf(accounts[0].getSnippet()));
    }
}
