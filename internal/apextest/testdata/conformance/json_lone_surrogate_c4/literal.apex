String lit = '\uD800';
System.debug('CAP|json_lit|' + JSON.serialize(new Map<String, Object>{'n' => lit.length(), 'u' => lit.getChars()}));
