public class QuoteTotalsProbe {
    static String ex(Exception e) { return 'EX ' + e.getTypeName() + ':' + e.getMessage(); }
    static String snap(Id qid) {
        Quote q = [SELECT Subtotal, TotalPrice, GrandTotal, Discount, Tax, ShippingHandling, LineItemCount FROM Quote WHERE Id = :qid];
        return 'sub=' + q.Subtotal + ' total=' + q.TotalPrice + ' grand=' + q.GrandTotal + ' disc=' + q.Discount + ' tax=' + q.Tax + ' ship=' + q.ShippingHandling + ' n=' + q.LineItemCount;
    }
    public static Map<String, Object> observe() {
        Map<String, Object> o = new Map<String, Object>();
        try {
            Id pbId = Test.isRunningTest() ? Test.getStandardPricebookId() : [SELECT Id FROM Pricebook2 WHERE IsStandard = true LIMIT 1].Id;
            Product2 p = new Product2(Name = 'QtProbe P', IsActive = true); insert p;
            PricebookEntry pbe = new PricebookEntry(Pricebook2Id = pbId, Product2Id = p.Id, UnitPrice = 100, IsActive = true); insert pbe;
            Account a = new Account(Name = 'QtProbe A'); insert a;
            Opportunity op = new Opportunity(Name = 'QtProbe O', AccountId = a.Id, StageName = 'Prospecting', CloseDate = Date.newInstance(2030, 1, 1), Pricebook2Id = pbId); insert op;
            Quote q = new Quote(Name = 'QtProbe Q', OpportunityId = op.Id, Pricebook2Id = pbId, ShippingHandling = 5, Tax = 3); insert q;
            o.put('empty', snap(q.Id));
            QuoteLineItem l1 = new QuoteLineItem(QuoteId = q.Id, PricebookEntryId = pbe.Id, Quantity = 2, UnitPrice = 95); insert l1;
            o.put('oneLine', snap(q.Id));
            QuoteLineItem l2 = new QuoteLineItem(QuoteId = q.Id, PricebookEntryId = pbe.Id, Quantity = 3, UnitPrice = 10, Discount = 10); insert l2;
            o.put('twoLines', snap(q.Id));
            QuoteLineItem r1 = [SELECT Subtotal, TotalPrice, ListPrice, UnitPrice, Quantity, Discount FROM QuoteLineItem WHERE Id = :l2.Id];
            o.put('line2', 'sub=' + r1.Subtotal + ' total=' + r1.TotalPrice + ' list=' + r1.ListPrice + ' unit=' + r1.UnitPrice + ' qty=' + r1.Quantity + ' disc=' + r1.Discount);
            update new QuoteLineItem(Id = l1.Id, Quantity = 4);
            o.put('afterUpdate', snap(q.Id));
            delete l2;
            o.put('afterDelete', snap(q.Id));
        } catch (Exception e) { o.put('error', ex(e)); }
        return o;
    }
}
