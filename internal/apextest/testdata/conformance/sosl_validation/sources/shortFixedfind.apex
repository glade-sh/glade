obs=String.valueOf(Search.find('FIND {x} RETURNING Account(Id)').get('Account').size());
