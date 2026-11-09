@IsTest
private class RtWs {
    static final List<String> OPTS = new List<String>{'OptionsAutoFacetingEnabled','OptionsCartAsyncProcessingEnabled','OptionsCartCalculateEnabled','OptionsCartToOrderAutoCustomFieldMapping','OptionsCommerceEinsteinActivitiesTracked','OptionsCommerceEinsteinDeployed','OptionsDuplicateCartItemsEnabled','OptionsGuestBrowsingEnabled','OptionsGuestCartEnabled','OptionsGuestCheckoutEnabled','OptionsPreserveGuestCartEnabled','OptionsSkipAdditionalEntitlementCheckForSearch','OptionsSkuDetectionEnabled','OptionsSplitShipmentEnabled'};
    @IsTest static void members() {
        List<String> out = new List<String>();
        Map<String,Schema.SObjectField> m = WebStore.SObjectType.getDescribe().fields.getMap();
        for (String n : OPTS) {
            Schema.SObjectField f = m.get(n);
            if (f == null) { out.add(n + '=MISSING'); continue; }
            Schema.DescribeFieldResult d = f.getDescribe();
            out.add(n + '=' + d.getType() + '|c' + d.isCreateable() + '|u' + d.isUpdateable() + '|n' + d.isNillable() + '|dflt' + d.isDefaultedOnCreate() + '|a' + d.isAccessible());
        }
        System.assert(false, String.join(out, ' ;; '));
    }
    @IsTest static void names_only() {
        List<String> out = new List<String>();
        try { WebStore s = new WebStore(Name='n1'); insert s; out.add('insert_name_only=OK'); }
        catch (DmlException e) { out.add('insert_name_only=' + e.getDmlStatusCode(0) + ':' + e.getDmlFields(0) + ':' + e.getDmlMessage(0)); }
        catch (Exception e) { out.add('insert_name_only=EXC ' + e.getTypeName() + ': ' + e.getMessage()); }
        System.assert(false, String.join(out, ' ;; '));
    }
    @IsTest static void dyn_put_all_false() {
        List<String> out = new List<String>();
        try {
            SObject s = Schema.getGlobalDescribe().get('WebStore').newSObject();
            s.put('Name', 'n2');
            for (String n : OPTS) s.put(n, false);
            insert s;
            String q = 'SELECT ' + String.join(OPTS, ',') + ' FROM WebStore WHERE Id = \'' + s.Id + '\'';
            SObject r = Database.query(q);
            List<String> vals = new List<String>();
            for (String n : OPTS) vals.add(String.valueOf(r.get(n)));
            out.add('insert_all_false=OK ' + String.join(vals, ','));
        } catch (Exception e) { out.add('insert_all_false=EXC ' + e.getTypeName() + ': ' + e.getMessage()); }
        System.assert(false, String.join(out, ' ;; '));
    }
    @IsTest static void dyn_put_mixed() {
        List<String> out = new List<String>();
        try {
            SObject s = Schema.getGlobalDescribe().get('WebStore').newSObject();
            s.put('Name', 'n3');
            for (Integer i = 0; i < OPTS.size(); i++) s.put(OPTS[i], Math.mod(i, 2) == 0);
            insert s;
            String q = 'SELECT ' + String.join(OPTS, ',') + ' FROM WebStore WHERE Id = \'' + s.Id + '\'';
            SObject r = Database.query(q);
            List<String> vals = new List<String>();
            for (String n : OPTS) vals.add(String.valueOf(r.get(n)));
            out.add('insert_mixed=OK ' + String.join(vals, ','));
        } catch (Exception e) { out.add('insert_mixed=EXC ' + e.getTypeName() + ': ' + e.getMessage()); }
        System.assert(false, String.join(out, ' ;; '));
    }
    @IsTest static void dyn_put_missing_one() {
        List<String> out = new List<String>();
        for (String skip : new List<String>{'OptionsGuestCartEnabled','OptionsAutoFacetingEnabled'}) {
            try {
                SObject s = Schema.getGlobalDescribe().get('WebStore').newSObject();
                s.put('Name', 'n4');
                for (String n : OPTS) if (n != skip) s.put(n, false);
                insert s;
                out.add('insert_missing_' + skip + '=OK');
            } catch (DmlException e) { out.add('insert_missing_' + skip + '=' + e.getDmlStatusCode(0) + ':' + e.getDmlFields(0) + ':' + e.getDmlMessage(0)); }
            catch (Exception e) { out.add('insert_missing_' + skip + '=EXC ' + e.getTypeName() + ': ' + e.getMessage()); }
        }
        System.assert(false, String.join(out, ' ;; '));
    }
    @IsTest static void put_unknown() {
        List<String> out = new List<String>();
        try { SObject s = Schema.getGlobalDescribe().get('WebStore').newSObject(); s.put('OptionsNoSuchField', false); out.add('put_unknown=OK'); }
        catch (Exception e) { out.add('put_unknown=EXC ' + e.getTypeName() + ': ' + e.getMessage()); }
        System.assert(false, String.join(out, ' ;; '));
    }
}
