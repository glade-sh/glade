@IsTest
private class RtCe {
    static SObject mk(String t) { return Schema.getGlobalDescribe().get(t).newSObject(); }
    @IsTest static void ce_members() {
        List<String> out = new List<String>();
        Map<String,List<String>> plan = new Map<String,List<String>>{
            'WebStoreBuyerGroup' => new List<String>{'WebStoreId','BuyerGroupId'},
            'BuyerGroup' => new List<String>{'Name','Description'},
            'CommerceEntitlementPolicy' => new List<String>{'Name','CanViewProduct','CanViewPrice','IsActive','Description'},
            'CommerceEntitlementBuyerGroup' => new List<String>{'PolicyId','BuyerGroupId'},
            'CommerceEntitlementProduct' => new List<String>{'PolicyId','ProductId'}};
        List<String> objs = new List<String>(plan.keySet()); objs.sort();
        for (String o : objs) {
            Map<String,Schema.SObjectField> m = Schema.getGlobalDescribe().get(o).getDescribe().fields.getMap();
            for (String f : plan.get(o)) {
                Schema.SObjectField sf = m.get(f);
                if (sf == null) { out.add(o + '.' + f + '=MISSING'); continue; }
                Schema.DescribeFieldResult d = sf.getDescribe();
                out.add(o + '.' + f + '=' + d.getType() + '|c' + d.isCreateable() + '|u' + d.isUpdateable() + '|n' + d.isNillable() + '|a' + d.isAccessible());
            }
        }
        System.assert(false, String.join(out, ' ;; '));
    }
    @IsTest static void ce_chain() {
        List<String> out = new List<String>();
        try {
            SObject store = mk('WebStore'); store.put('Name', 's1'); insert store;
            SObject bg = mk('BuyerGroup'); bg.put('Name', 'bg1'); insert bg;
            SObject link = mk('WebStoreBuyerGroup'); link.put('WebStoreId', store.Id); link.put('BuyerGroupId', bg.Id); insert link;
            SObject r = Database.query('SELECT WebStoreId, BuyerGroupId FROM WebStoreBuyerGroup WHERE Id = \'' + link.Id + '\'');
            out.add('store_group_link=' + (r.get('WebStoreId') == store.Id) + ',' + (r.get('BuyerGroupId') == bg.Id));
            SObject pol = mk('CommerceEntitlementPolicy'); pol.put('Name', 'p1'); pol.put('CanViewProduct', true); pol.put('CanViewPrice', false); insert pol;
            SObject pr = Database.query('SELECT Name, CanViewProduct, CanViewPrice, IsActive FROM CommerceEntitlementPolicy WHERE Id = \'' + pol.Id + '\'');
            out.add('policy_values=' + pr.get('Name') + ',' + pr.get('CanViewProduct') + ',' + pr.get('CanViewPrice') + ',' + pr.get('IsActive'));
            SObject pbg = mk('CommerceEntitlementBuyerGroup'); pbg.put('PolicyId', pol.Id); pbg.put('BuyerGroupId', bg.Id); insert pbg;
            Product2 p = new Product2(Name='prod', IsActive=true); insert p;
            SObject pp = mk('CommerceEntitlementProduct'); pp.put('PolicyId', pol.Id); pp.put('ProductId', p.Id); insert pp;
            SObject pr2 = Database.query('SELECT PolicyId, ProductId FROM CommerceEntitlementProduct WHERE Id = \'' + pp.Id + '\'');
            out.add('policy_product=' + (pr2.get('PolicyId') == pol.Id) + ',' + (pr2.get('ProductId') == p.Id));
            SObject pbr = Database.query('SELECT PolicyId, BuyerGroupId FROM CommerceEntitlementBuyerGroup WHERE Id = \'' + pbg.Id + '\'');
            out.add('policy_buyergroup=' + (pbr.get('PolicyId') == pol.Id) + ',' + (pbr.get('BuyerGroupId') == bg.Id));
        } catch (Exception e) { out.add('chain=EXC ' + e.getTypeName() + ': ' + e.getMessage()); }
        System.assert(false, String.join(out, ' ;; '));
    }
    @IsTest static void ce_policy_name_only() {
        List<String> out = new List<String>();
        try { SObject pol = mk('CommerceEntitlementPolicy'); pol.put('Name', 'p2'); insert pol;
            SObject pr = Database.query('SELECT CanViewProduct, CanViewPrice, IsActive FROM CommerceEntitlementPolicy WHERE Id = \'' + pol.Id + '\'');
            out.add('policy_name_only=OK ' + pr.get('CanViewProduct') + ',' + pr.get('CanViewPrice') + ',' + pr.get('IsActive'));
        } catch (Exception e) { out.add('policy_name_only=EXC ' + e.getTypeName() + ': ' + e.getMessage()); }
        System.assert(false, String.join(out, ' ;; '));
    }
}
