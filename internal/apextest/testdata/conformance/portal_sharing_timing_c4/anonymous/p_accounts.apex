for (String lic : new List<String>{'cc', 'ccp'}) {
    Account parent = new Account(Name = '__P__' + lic + 'Parent');
    insert parent;
    insert new List<Account>{new Account(Name = '__P__' + lic + 'Child1', ParentId = parent.Id), new Account(Name = '__P__' + lic + 'Child2', ParentId = parent.Id)};
    insert new Contact(LastName = '__P__' + lic + 'Contact', AccountId = parent.Id);
}
System.debug('CAP|p_accounts|ok');
