List<Account> accts = new List<Account>{
    new Account(Name = 'Copper Wire qzx', Website = 'wire.example'),
    new Account(Name = 'copper silver qzx'),
    new Account(Name = 'Coppers qzx'),
    new Account(Name = 'Nfx Alpha qzx', Website = 'copper.example'),
    new Account(Name = 'copper beta qzx'),
    new Account(Name = 'Nfx Gamma qzx', Site = 'copper-site'),
    new Account(Name = 'zork one qzx'),
    new Account(Name = 'zork two qzx'),
    new Account(Name = 'zork one two qzx'),
    new Account(Name = 'quux one qzx'),
    new Account(Name = 'quux two qzx'),
    new Account(Name = 'quux qzx'),
    new Account(Name = 'brk{ley qzx'),
    new Account(Name = 'quo"te qzx'),
    new Account(Name = 'col:on qzx'),
    new Account(Name = 'back\\slash qzx'),
    new Account(Name = 'q?mark qzx'),
    new Account(Name = 'st*ar qzx')
};
insert accts;
insert new Contact(LastName = 'Cznx qzx', AssistantName = 'copper');
insert new Lead(LastName = 'Lznx qzx', Company = 'copper');
insert new Case(Subject = 'copper qzx');
insert new Task(Subject = 'copper qzx', Status = 'Not Started', Priority = 'Normal');
List<Document> docs = new List<Document>{
    new Document(Name = 'zork only qzx', DeveloperName = 'zdoc1', FolderId = UserInfo.getUserId(), Body = Blob.valueOf('x')),
    new Document(Name = 'one only qzx', DeveloperName = 'zdoc2', FolderId = UserInfo.getUserId(), Body = Blob.valueOf('x')),
    new Document(Name = 'zork one qzx', DeveloperName = 'zdoc3', FolderId = UserInfo.getUserId(), Body = Blob.valueOf('x'))
};
insert docs;
