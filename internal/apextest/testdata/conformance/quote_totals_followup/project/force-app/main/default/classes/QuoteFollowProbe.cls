public class QuoteFollowProbe {
    static String ex(Exception e) { return 'EX ' + e.getTypeName() + ':' + e.getMessage(); }
    static String snap(Id qid) {
        Quote q = [SELECT Subtotal, TotalPrice, GrandTotal, Discount, Tax, ShippingHandling, LineItemCount FROM Quote WHERE Id = :qid];
        return 'sub=' + q.Subtotal + ' total=' + q.TotalPrice + ' grand=' + q.GrandTotal + ' disc=' + q.Discount + ' tax=' + q.Tax + ' ship=' + q.ShippingHandling + ' n=' + q.LineItemCount;
    }
    static String line(Id lid) {
        QuoteLineItem r = [SELECT Subtotal, TotalPrice, ListPrice, UnitPrice, Quantity, Discount FROM QuoteLineItem WHERE Id = :lid];
        return 'sub=' + r.Subtotal + ' total=' + r.TotalPrice + ' list=' + r.ListPrice + ' unit=' + r.UnitPrice + ' qty=' + r.Quantity + ' disc=' + r.Discount;
    }
    public static Map<String, Object> observe() {
        Map<String, Object> o = new Map<String, Object>();
        try {
            Id pbId = Test.isRunningTest() ? Test.getStandardPricebookId() : [SELECT Id FROM Pricebook2 WHERE IsStandard = true LIMIT 1].Id;
            Product2 p = new Product2(Name = 'QtFollow P', IsActive = true); insert p;
            PricebookEntry pbe = new PricebookEntry(Pricebook2Id = pbId, Product2Id = p.Id, UnitPrice = 100, IsActive = true); insert pbe;
            Account a = new Account(Name = 'QtFollow A'); insert a;
            Opportunity op = new Opportunity(Name = 'QtFollow O', AccountId = a.Id, StageName = 'Prospecting', CloseDate = Date.newInstance(2030, 1, 1), Pricebook2Id = pbId); insert op;
            Quote q = new Quote(Name = 'QtFollow Q', OpportunityId = op.Id, Pricebook2Id = pbId); insert q;
            o.put('noTaxEmpty', snap(q.Id));
            QuoteLineItem l1 = new QuoteLineItem(QuoteId = q.Id, PricebookEntryId = pbe.Id, Quantity = 2, UnitPrice = 100); insert l1;
            o.put('noTaxOne', snap(q.Id));
            QuoteLineItem l2 = new QuoteLineItem(QuoteId = q.Id, PricebookEntryId = pbe.Id, Quantity = 1, UnitPrice = 100, Discount = 50); insert l2;
            o.put('quarter', snap(q.Id));
            o.put('quarterLine', line(l2.Id));
            update new Quote(Id = q.Id, Tax = 7.5, ShippingHandling = 2.25);
            o.put('quoteUpdate', snap(q.Id));
            update new QuoteLineItem(Id = l2.Id, Discount = 20);
            o.put('discUpdate', snap(q.Id));
            o.put('discUpdateLine', line(l2.Id));
            update new QuoteLineItem(Id = l2.Id, UnitPrice = 50, Quantity = 3);
            o.put('priceUpdateLine', line(l2.Id));
            o.put('priceUpdate', snap(q.Id));
            update new QuoteLineItem(Id = l2.Id, Discount = null);
            o.put('discNullLine', line(l2.Id));
            update new QuoteLineItem(Id = l2.Id, Discount = 100);
            o.put('discFullLine', line(l2.Id));
            o.put('discFull', snap(q.Id));
            delete l2;
            o.put('deleted', snap(q.Id));
            undelete l2;
            o.put('undeleted', snap(q.Id));
            o.put('undeletedLine', line(l2.Id));
            delete l1; delete l2;
            o.put('allDeleted', snap(q.Id));
            update new Quote(Id = q.Id, Tax = null, ShippingHandling = null);
            o.put('nullTaxShip', snap(q.Id));
            try { SObject w1 = new Quote(Id = q.Id); w1.put('GrandTotal', 5); update w1; o.put('writeGrand', 'ok'); } catch (Exception e) { o.put('writeGrand', ex(e)); }
            try { SObject w2 = new QuoteLineItem(Id = l1.Id); w2.put('Subtotal', 5); update w2; o.put('writeLineSub', 'ok'); } catch (Exception e) { o.put('writeLineSub', ex(e)); }
        } catch (Exception e) { o.put('error', ex(e)); }
        return o;
    }
}
