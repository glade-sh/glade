public with sharing class AddressFieldProbe {
    static List<String> out;
    static void row(String id, Object value) {
        out.add(id + '|' + String.valueOf(value));
    }
    static String err(Exception e) {
        String text = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
        if (e instanceof DmlException) {
            DmlException d = (DmlException) e;
            text += '|' + d.getDmlType(0) + '|' + String.valueOf(d.getDmlFieldNames(0));
        }
        return text;
    }
    static String addr(Address a) {
        if (a == null) {
            return 'null';
        }
        return a.getStreet() + '/' + a.getCity() + '/' + a.getPostalCode() + '/' + a.getState() + '/' + a.getStateCode()
            + '/' + a.getCountry() + '/' + a.getCountryCode() + '/' + a.getLatitude() + '/' + a.getLongitude() + '/' + a.getGeocodeAccuracy();
    }
    static String fieldFacets(Schema.DescribeFieldResult d) {
        return d.getName() + ',' + d.getType() + ',' + d.getSoapType() + ',' + d.getCompoundFieldName() + ',' + d.isCreateable() + ','
            + d.isUpdateable() + ',' + d.isNillable() + ',' + d.getLength() + ',' + d.getPrecision() + ',' + d.getScale() + ','
            + d.isFilterable() + ',' + d.isSortable() + ',' + d.isGroupable() + ',' + d.isCustom() + ',' + d.getLabel() + ','
            + String.valueOf(d.getController()) + ',' + d.isRestrictedPicklist() + ',' + d.getPicklistValues().size();
    }
    public static List<String> run() {
        out = new List<String>();
        Addr_Test__c a = new Addr_Test__c(Name = 'one');
        a.Home__Street__s = '1 Main St';
        a.Home__City__s = 'Springfield';
        a.Home__PostalCode__s = '94105';
        a.Home__StateCode__s = 'CA';
        a.Home__CountryCode__s = 'US';
        a.Home__Latitude__s = 37.5;
        a.Home__Longitude__s = -122.25;
        row('P01', a.Home__City__s);
        row('P02', addr(a.Home__c));
        row('P02b', a.Home__Latitude__s);
        row('P02c', a.home__city__s);
        try { insert a; row('P03', 'inserted'); } catch (Exception e) { row('P03', err(e)); }
        Addr_Test__c q = [SELECT Id, Home__Street__s, Home__City__s, Home__PostalCode__s, Home__StateCode__s, Home__CountryCode__s,
            Home__Latitude__s, Home__Longitude__s, Home__GeocodeAccuracy__s, Home__c FROM Addr_Test__c WHERE Id = :a.Id];
        row('P04', q.Home__Street__s + '/' + q.Home__City__s + '/' + q.Home__PostalCode__s + '/' + q.Home__StateCode__s + '/'
            + q.Home__CountryCode__s + '/' + q.Home__Latitude__s + '/' + q.Home__Longitude__s + '/' + q.Home__GeocodeAccuracy__s);
        row('P05', addr(q.Home__c));
        Addr_Test__c onlyCompound = [SELECT Id, Home__c FROM Addr_Test__c WHERE Id = :a.Id];
        row('P06', addr(onlyCompound.Home__c));
        try { row('P07', onlyCompound.Home__City__s); } catch (Exception e) { row('P07', err(e)); }
        row('P07b', new List<String>(onlyCompound.getPopulatedFieldsAsMap().keySet()));
        Addr_Test__c onlyComponents = [SELECT Id, Home__City__s, Home__CountryCode__s FROM Addr_Test__c WHERE Id = :a.Id];
        try { row('P08', addr(onlyComponents.Home__c)); } catch (Exception e) { row('P08', err(e)); }
        row('P08b', new List<String>(onlyComponents.getPopulatedFieldsAsMap().keySet()));
        row('P09', [SELECT COUNT() FROM Addr_Test__c WHERE Home__City__s = 'Springfield']);
        row('P09b', [SELECT COUNT() FROM Addr_Test__c WHERE Home__CountryCode__s = 'US' AND Home__Latitude__s > 37]);
        try { row('P10', Database.query('SELECT COUNT() FROM Addr_Test__c WHERE Home__c != null')); } catch (Exception e) { row('P10', err(e)); }
        try { row('P10c', Database.query('SELECT Id FROM Addr_Test__c WHERE home__city__s = \'Springfield\' ORDER BY Home__City__s').size()); } catch (Exception e) { row('P10c', err(e)); }
        try { row('P10d', Database.query('SELECT Home__City__s c, COUNT(Id) n FROM Addr_Test__c GROUP BY Home__City__s')); } catch (Exception e) { row('P10d', err(e)); }
        try { row('P10e', Database.query('SELECT Id FROM Addr_Test__c WHERE DISTANCE(Home__c, GEOLOCATION(37.5, -122.25), \'km\') < 1').size()); } catch (Exception e) { row('P10e', err(e)); }
        try { row('P10f', Database.query('SELECT Home__State__s FROM Addr_Test__c').size()); } catch (Exception e) { row('P10f', err(e)); }
        SObject s = q;
        row('P11', s.get('Home__City__s'));
        row('P11b', s.get('home__city__s'));
        Object compound = s.get('Home__c');
        row('P11c', compound instanceof Address);
        row('P11d', s.get('Home__Latitude__s') instanceof Decimal);
        try { s.put('Home__City__s', 'Shelbyville'); row('P12', q.Home__City__s); } catch (Exception e) { row('P12', err(e)); }
        try { s.put('Home__c', compound); row('P12b', 'put'); } catch (Exception e) { row('P12b', err(e)); }
        try { s.put('Home__c', null); row('P12c', 'put'); } catch (Exception e) { row('P12c', err(e)); }
        try { row('P12d', s.get('Home__Bogus__s')); } catch (Exception e) { row('P12d', err(e)); }
        try { update q; row('P13', [SELECT Home__City__s FROM Addr_Test__c WHERE Id = :a.Id].Home__City__s); } catch (Exception e) { row('P13', err(e)); }
        Addr_Test__c j = [SELECT Home__City__s, Home__CountryCode__s, Home__c FROM Addr_Test__c WHERE Id = :a.Id];
        Map<String, Object> m = (Map<String, Object>) JSON.deserializeUntyped(JSON.serialize(j));
        m.remove('Id');
        m.remove('attributes');
        List<String> keys = new List<String>(m.keySet());
        keys.sort();
        List<String> parts = new List<String>();
        for (String k : keys) {
            parts.add(k + '=' + JSON.serialize(m.get(k)));
        }
        row('P14', String.join(parts, ';'));
        try { insert new Addr_Test__c(Name = 'bad', Home__CountryCode__s = 'ZZ'); row('P15', 'inserted'); } catch (Exception e) { row('P15', err(e)); }
        try { insert new Addr_Test__c(Name = 'bad', Home__StateCode__s = 'CA'); row('P16', 'inserted'); } catch (Exception e) { row('P16', err(e)); }
        try { insert new Addr_Test__c(Name = 'bad', Home__City__s = 'x'.repeat(41)); row('P17', 'inserted'); } catch (Exception e) { row('P17', err(e)); }
        try { insert new Addr_Test__c(Name = 'bad', Home__Latitude__s = 91); row('P18', 'inserted'); } catch (Exception e) { row('P18', err(e)); }
        try { insert new Addr_Test__c(Name = 'bad', Home__GeocodeAccuracy__s = 'Bogus'); row('P19', 'inserted'); } catch (Exception e) { row('P19', err(e)); }
        try { insert new Addr_Test__c(Name = 'bad', Home__StateCode__s = 'XX', Home__CountryCode__s = 'US'); row('P20', 'inserted'); } catch (Exception e) { row('P20', err(e)); }
        try { insert new Addr_Test__c(Name = 'bad', Home__Latitude__s = 10); row('P21', 'inserted'); } catch (Exception e) { row('P21', err(e)); }
        try { insert new Addr_Test__c(Name = 'ok', Home__GeocodeAccuracy__s = 'Street', Home__Street__s = 'x'.repeat(255)); row('P22', 'inserted'); } catch (Exception e) { row('P22', err(e)); }
        Map<String, Schema.SObjectField> fields = Schema.SObjectType.Addr_Test__c.fields.getMap();
        List<String> names = new List<String>();
        for (String k : fields.keySet()) {
            if (k.startsWith('home')) names.add(k);
        }
        names.sort();
        row('D01', names);
        for (String k : names) {
            row('D02.' + k, fieldFacets(fields.get(k).getDescribe()));
        }
        row('D03', String.valueOf(Schema.Addr_Test__c.Home__City__s));
        row('D04', fieldFacets(Addr_Test__c.Home__c.getDescribe()));
        Account acc = new Account(Name = 'acct', Home__City__s = 'Kelowna', Home__CountryCode__s = 'CA', Home__StateCode__s = 'BC', Home__Street__s = '2 Side');
        try { insert acc; row('A01', 'inserted'); } catch (Exception e) { row('A01', err(e)); }
        Account qa = [SELECT Home__City__s, Home__StateCode__s, Home__c, BillingAddress FROM Account WHERE Id = :acc.Id];
        row('A02', qa.Home__City__s + '/' + qa.Home__StateCode__s + '/' + addr(qa.Home__c) + '/' + addr(qa.BillingAddress));
        row('A03', fieldFacets(Account.Home__Street__s.getDescribe()));
        row('A04', [SELECT COUNT() FROM Account WHERE Home__StateCode__s = 'BC']);
        return out;
    }
}
