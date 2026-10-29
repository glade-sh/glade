public with sharing class AddressFollowup {
    static List<String> out;
    static void row(String id, Object value) { out.add(id + '|' + String.valueOf(value)); }
    static String err(Exception e) {
        String text = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
        if (e instanceof DmlException) {
            DmlException d = (DmlException) e;
            text += '|' + d.getDmlType(0) + '|' + String.valueOf(d.getDmlFieldNames(0));
        }
        return text;
    }
    static void ins(String id, SObject o) { try { insert o; row(id, 'inserted'); } catch (Exception e) { row(id, err(e)); } }
    static void upd(String id, SObject o) { try { update o; row(id, 'updated'); } catch (Exception e) { row(id, err(e)); } }
    static Addr_Test__c fresh() {
        Addr_Test__c a = new Addr_Test__c(Name = 'base', Home__City__s = 'Springfield', Home__StateCode__s = 'CA', Home__CountryCode__s = 'US', Home__Latitude__s = 37.5, Home__Longitude__s = -122.25);
        insert a;
        return a;
    }
    public static List<String> run() {
        out = new List<String>();
        // J: JSON of standard and custom addresses.
        Account a1 = new Account(Name = 'ja1', BillingStreet = '1 Main St', BillingCity = 'Springfield', BillingPostalCode = '94105', BillingState = 'California', BillingCountry = 'United States', BillingLatitude = 37.5, BillingLongitude = -122.25, ShippingCity = 'Shelbyville');
        insert a1;
        Account a2 = new Account(Name = 'ja2', BillingCity = 'Solo');
        insert a2;
        Account q1 = [SELECT Id, BillingAddress, ShippingAddress FROM Account WHERE Id = :a1.Id];
        Account q2 = [SELECT Id, BillingAddress FROM Account WHERE Id = :a2.Id];
        row('J01', JSON.serialize(q1.BillingAddress));
        row('J02', JSON.serialize(q2.BillingAddress));
        row('J03', JSON.serialize(q1.ShippingAddress));
        row('J04', JSON.serializePretty(q2.BillingAddress));
        row('J05', JSON.serialize(q2.BillingAddress, true));
        row('J06', JSON.serialize(new Map<String, Object>{'k' => q2.BillingAddress}));
        row('J07', JSON.serialize(q2).replace(q2.Id, 'ID'));
        row('J07b', JSON.serialize(q1).replace(q1.Id, 'ID'));
        row('J07c', JSON.serialize(q2, true).replace(q2.Id, 'ID'));
        Addr_Test__c h = new Addr_Test__c(Name = 'jh', Home__City__s = 'Solo');
        insert h;
        Addr_Test__c qh = [SELECT Id, Home__c FROM Addr_Test__c WHERE Id = :h.Id];
        row('J08', JSON.serialize(qh.Home__c));
        row('J09', JSON.serialize(qh.Home__c, true));
        row('J10', JSON.serialize(qh).replace(qh.Id, 'ID'));
        row('J11', JSON.serialize(qh, true).replace(qh.Id, 'ID'));
        // R: restricted picklist messages.
        ins('R01', new Account(Name = 'r', BillingGeocodeAccuracy = 'Bogus'));
        ins('R02', new Opportunity(Name = 'r', StageName = 'Bogus', CloseDate = Date.today()));
        ins('R03', new Addr_Test__c(Name = 'r', Pick__c = 'Z'));
        ins('R04', new Addr_Test__c(Name = 'r', Ctl__c = 'X', Dep__c = 'Z'));
        ins('R05', new Addr_Test__c(Name = 'r', Ctl__c = 'X', Dep__c = 'D2'));
        ins('R06', new Addr_Test__c(Name = 'r', Dep__c = 'D1'));
        ins('R07', new Addr_Test__c(Name = 'r', Ctl__c = 'X', Dep__c = 'D1'));
        ins('R08', new Addr_Test__c(Name = 'r', Pick__c = 'a'));
        ins('R09', new Addr_Test__c(Name = 'r', Pick__c = ' '));
        ins('R10', new Addr_Test__c(Name = 'r', Home__GeocodeAccuracy__s = 'Bogus'));
        ins('R11', new Addr_Test__c(Name = 'r', Home__CountryCode__s = 'ZZ'));
        ins('R12', new Account(Name = 'r', Home__GeocodeAccuracy__s = 'Bogus'));
        ins('R13', new Lead(LastName = 'r', Company = 'r', Status = 'Bogus'));
        ins('R14', new Case(Subject = 'r', Status = 'Bogus'));
        ins('R15', new Account(Name = 'r', Type = 'Bogus'));
        ins('R16', new Account(Name = 'r', Rating = 'Bogus'));
        Addr_Test__c b = fresh();
        b.Pick__c = 'Z'; upd('R20', b);
        Addr_Test__c b2 = fresh();
        b2.Dep__c = 'Z'; upd('R21', b2);
        Addr_Test__c b3 = fresh();
        b3.Dep__c = 'D1'; upd('R22', b3);
        Addr_Test__c b4 = fresh();
        b4.Home__GeocodeAccuracy__s = 'Bogus'; upd('R23', b4);
        // L: latitude / longitude insert.
        Decimal[] lats = new Decimal[]{-91, 90, -90, 90.5, -90.5, 0};
        for (Decimal d : lats) { ins('L01.' + d, new Addr_Test__c(Name = 'l', Home__Latitude__s = d, Home__Longitude__s = 0)); }
        Decimal[] lons = new Decimal[]{180, -180, 181, -181, 180.5, 0};
        for (Decimal d : lons) { ins('L02.' + d, new Addr_Test__c(Name = 'l', Home__Latitude__s = 0, Home__Longitude__s = d)); }
        ins('L03', new Addr_Test__c(Name = 'l', Home__Longitude__s = 10));
        ins('L03b', new Addr_Test__c(Name = 'l', Home__Longitude__s = 181));
        ins('L03c', new Addr_Test__c(Name = 'l', Home__Latitude__s = 91));
        ins('L03d', new Addr_Test__c(Name = 'l', Home__Latitude__s = -91, Home__Longitude__s = 181));
        ins('L03e', new Addr_Test__c(Name = 'l', Home__Latitude__s = 10, Home__Longitude__s = 181));
        // U: update route against an existing record (lat 37.5, lon -122.25, CA/US).
        Addr_Test__c u;
        u = fresh(); u.Home__Latitude__s = 91; upd('U02', u);
        u = fresh(); u.Home__Latitude__s = -91; upd('U03', u);
        u = fresh(); u.Home__Latitude__s = 90; upd('U04', u);
        u = fresh(); u.Home__Longitude__s = 181; upd('U05', u);
        u = fresh(); u.Home__Longitude__s = -181; upd('U06', u);
        u = fresh(); u.Home__Longitude__s = 180; upd('U07', u);
        u = fresh(); u.Home__Longitude__s = null; upd('U08', u);
        u = fresh(); u.Home__Latitude__s = null; upd('U09', u);
        u = fresh(); u.Home__Latitude__s = null; u.Home__Longitude__s = null; upd('U10', u);
        u = fresh(); u.Home__CountryCode__s = 'ZZ'; upd('U11', u);
        u = fresh(); u.Home__StateCode__s = 'XX'; upd('U12', u);
        u = fresh(); u.Home__CountryCode__s = 'CA'; upd('U13', u);
        u = fresh(); u.Home__CountryCode__s = null; upd('U14', u);
        u = fresh(); u.Home__City__s = 'y'.repeat(41); upd('U15', u);
        u = fresh(); u.Home__StateCode__s = null; upd('U16', u);
        // U17-U19: a partial update through a re-queried record.
        u = fresh();
        Addr_Test__c pq = new Addr_Test__c(Id = u.Id, Home__Latitude__s = 91);
        upd('U17', pq);
        pq = new Addr_Test__c(Id = u.Id, Home__StateCode__s = 'XX');
        upd('U18', pq);
        pq = new Addr_Test__c(Id = u.Id, Home__Longitude__s = null);
        upd('U19', pq);
        pq = new Addr_Test__c(Id = u.Id, Home__CountryCode__s = null);
        upd('U20', pq);
        u = fresh(); u.Home__Longitude__s = null; u.Home__Latitude__s = 10; upd('U21', u);
        return out;
    }
}
