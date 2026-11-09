@IsTest
private class RtOrd {
    private static String run(String id, Callable c) { return null; }
    public interface Callable { Object call(String a, Map<String,Object> b); }
    @IsTest static void members() {
        List<String> out = new List<String>();
        Map<String,Schema.SObjectField> om = Order.SObjectType.getDescribe().fields.getMap();
        Map<String,Schema.SObjectField> im = OrderItem.SObjectType.getDescribe().fields.getMap();
        out.add('has_OrderedDate=' + om.containsKey('ordereddate'));
        out.add('has_TotalLineAmount=' + im.containsKey('totallineamount'));
        for (String n : new List<String>{'ordereddate','totallineamount'}) {
            Schema.SObjectField f = om.containsKey(n) ? om.get(n) : im.get(n);
            if (f != null) { Schema.DescribeFieldResult d = f.getDescribe();
                out.add(n + '_desc=' + d.getName() + '|' + d.getType() + '|c' + d.isCreateable() + '|u' + d.isUpdateable() + '|n' + d.isNillable() + '|calc' + d.isCalculated() + '|len' + d.getLength() + '|p' + d.getPrecision() + '|s' + d.getScale() + '|label ' + d.getLabel()); }
        }
        System.assert(false, String.join(out, ' ;; '));
    }
    @IsTest static void dml() {
        List<String> out = new List<String>();
        Account a = new Account(Name='a'); insert a;
        Id pbId = Test.getStandardPricebookId();
        Product2 p = new Product2(Name='p', IsActive=true); insert p;
        PricebookEntry pbe = new PricebookEntry(Pricebook2Id=pbId, Product2Id=p.Id, UnitPrice=10, IsActive=true); insert pbe;
        try {
            Order o = new Order(AccountId=a.Id, EffectiveDate=Date.newInstance(2023,1,2), Status='Draft', Pricebook2Id=pbId, OrderedDate=Date.newInstance(2023,1,1));
            insert o;
            Order r = [SELECT Id, OrderedDate FROM Order WHERE Id = :o.Id];
            out.add('order_date_value=' + r.OrderedDate);
        } catch (Exception e) { out.add('order_date_value=EXC ' + e.getTypeName() + ': ' + e.getMessage()); }
        try {
            Order o = new Order(AccountId=a.Id, EffectiveDate=Date.newInstance(2023,1,2), Status='Draft', Pricebook2Id=pbId, OrderedDate=Datetime.newInstanceGmt(2023,1,1,5,6,7));
            insert o;
            Order r = [SELECT Id, OrderedDate FROM Order WHERE Id = :o.Id];
            out.add('order_datetime_value=' + r.OrderedDate);
        } catch (Exception e) { out.add('order_datetime_value=EXC ' + e.getTypeName() + ': ' + e.getMessage()); }
        try {
            Order o = new Order(AccountId=a.Id, EffectiveDate=Date.newInstance(2023,1,2), Status='Draft', Pricebook2Id=pbId);
            insert o;
            Order r = [SELECT Id, OrderedDate FROM Order WHERE Id = :o.Id];
            out.add('order_default_value=' + r.OrderedDate);
            OrderItem oi = new OrderItem(OrderId=o.Id, PricebookEntryId=pbe.Id, Quantity=2, UnitPrice=10, TotalLineAmount=20);
            insert oi;
            OrderItem ri = [SELECT Id, TotalLineAmount, TotalPrice FROM OrderItem WHERE Id = :oi.Id];
            out.add('item_total_line_amount=' + ri.TotalLineAmount + '|total_price=' + ri.TotalPrice);
        } catch (Exception e) { out.add('item_total_line_amount=EXC ' + e.getTypeName() + ': ' + e.getMessage()); }
        try {
            Order o = new Order(AccountId=a.Id, EffectiveDate=Date.newInstance(2023,1,2), Status='Draft', Pricebook2Id=pbId);
            insert o;
            OrderItem oi = new OrderItem(OrderId=o.Id, PricebookEntryId=pbe.Id, Quantity=2, UnitPrice=10);
            insert oi;
            OrderItem ri = [SELECT Id, TotalLineAmount FROM OrderItem WHERE Id = :oi.Id];
            out.add('item_default_total_line_amount=' + ri.TotalLineAmount);
        } catch (Exception e) { out.add('item_default_total_line_amount=EXC ' + e.getTypeName() + ': ' + e.getMessage()); }
        System.assert(false, String.join(out, ' ;; '));
    }
}
