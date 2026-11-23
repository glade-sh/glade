@IsTest
private class R5CartProbe {
  private static Id setupCart() {
    Account a = new Account(Name='r5'); insert a;
    WebStore ws = new WebStore(Name='r5store'); insert ws;
    WebCart c = new WebCart(Name='r5cart', AccountId=a.Id, WebStoreId=ws.Id); insert c;
    return c.Id;
  }
  @IsTest static void fakeProduct() {
    String msg;
    try {
      Id cid = setupCart();
      CartDeliveryGroup g = new CartDeliveryGroup(Name='g', CartId=cid); insert g;
      CartItem i = new CartItem(Name='i', CartId=cid, CartDeliveryGroupId=g.Id, Quantity=1, Product2Id='01txx0000006lmmAAA', SalesPrice=1, Type='Product');
      insert i; msg = 'inserted';
    } catch (Exception e) { msg = e.getTypeName() + '|' + e.getMessage(); }
    System.assert(false, 'R5|fakeProduct|' + msg);
  }
  @IsTest static void emptyAddress() {
    String msg;
    try {
      CartExtension.Cart cart = CartExtension.CartTestUtil.createCart();
      CartExtension.CartDeliveryGroup g = cart.getCartDeliveryGroups().get(0);
      Address a = g.getDeliverToAddress();
      msg = 'addrNull=' + (a == null) + (a == null ? '' : '|state=' + a.getState() + '|country=' + a.getCountry());
    } catch (Exception e) { msg = e.getTypeName() + '|' + e.getMessage(); }
    System.assert(false, 'R5|emptyAddress|' + msg);
  }
}
