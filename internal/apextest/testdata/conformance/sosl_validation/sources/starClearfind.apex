obs=String.valueOf(Search.find('FIND {*} RETURNING Account(Id)').get('Account').size());
