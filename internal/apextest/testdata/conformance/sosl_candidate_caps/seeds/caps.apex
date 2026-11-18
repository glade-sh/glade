// repeat: 1
Account host = new Account(Name = 'capz host');
insert host;
List<Account> a = new List<Account>(); List<Contact> c = new List<Contact>(); List<Lead> l = new List<Lead>();
List<Opportunity> o = new List<Opportunity>(); List<Case> cs = new List<Case>(); List<Campaign> cm = new List<Campaign>();
List<Product2> p = new List<Product2>(); List<Asset> assetRows = new List<Asset>(); List<Event> ev = new List<Event>(); List<Task> t = new List<Task>();
for (Integer i = 0; i < 300; i++) {
    a.add(new Account(Name = 'capz' + i));
    c.add(new Contact(LastName = 'capz' + i));
    l.add(new Lead(LastName = 'capz' + i, Company = 'capzco'));
    o.add(new Opportunity(Name = 'capz' + i, StageName = 'Prospecting', CloseDate = Date.newInstance(2030, 1, 1)));
    cs.add(new Case(Subject = 'capz' + i));
    cm.add(new Campaign(Name = 'capz' + i));
    p.add(new Product2(Name = 'capz' + i));
    assetRows.add(new Asset(Name = 'capz' + i, AccountId = host.Id));
    ev.add(new Event(Subject = 'capz' + i, DurationInMinutes = 30, ActivityDateTime = DateTime.newInstance(2030, 1, 1, 12, 0, 0)));
    t.add(new Task(Subject = 'capz' + i, Status = 'Not Started', Priority = 'Normal'));
}
insert a; insert c; insert l; insert o; insert cs; insert cm; insert p; insert assetRows; insert t;
insert ev;
