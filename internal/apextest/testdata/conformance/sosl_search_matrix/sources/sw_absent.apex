Search.SearchResults sr = Search.find('FIND {copper} RETURNING Account(Name)');
obs = '' + sr.get('Contact').size();
