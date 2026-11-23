try {
  Account a = new Account(Name='r5'); insert a;
  WebStore ws = new WebStore(Name='r5store'); insert ws;
  WebCart c = new WebCart(Name='r5cart', AccountId=a.Id, WebStoreId=ws.Id); insert c;
  CartDeliveryGroup g = new CartDeliveryGroup(Name='g', CartId=c.Id); insert g;
  CartItem i = new CartItem(Name='i', CartId=c.Id, CartDeliveryGroupId=g.Id, Quantity=1, Product2Id='01t000000000001AAA', SalesPrice=1, Type='Product');
  insert i; System.debug('R5|fakeProduct|inserted');
} catch (Exception e) { System.debug('R5|fakeProduct|' + e.getTypeName() + '|' + e.getMessage()); }
