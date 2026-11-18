Search.SearchResults sr = Search.find('FIND {copper} RETURNING Account(Name) WITH SNIPPET');
String r = '';
for (Search.SearchResult x : sr.get('Account')) { String n = (String) x.getSObject().get('Name'); if (n != null && n.endsWith('qzx') && n.startsWith('Copper Wire')) { r = x.getSnippet('Name'); } }
obs = r;
