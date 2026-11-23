@IsTest
private class R5PoProbe {
  public class VP extends CartExtension.CheckoutPlaceOrder {
    public virtual override CartExtension.PlaceOrderResponse validate(CartExtension.PlaceOrderRequest r, List<String> domainList) {
      return super.validate(r, domainList);
    }
  }
  private static CartExtension.Cart mk() {
    Account a = new Account(Name = 'acc'); insert a;
    WebStore ws = new WebStore(Name = 'ws'); insert ws;
    WebCart wc = new WebCart(Name = 'cart', WebStoreId = ws.Id, AccountId = a.Id); insert wc;
    CartDeliveryGroup g = new CartDeliveryGroup(Name = 'grp', CartId = wc.Id); insert g;
    Product2 p = new Product2(Name = 'p', IsActive = true); insert p;
    insert new CartItem(Name = 'ci', SalesPrice = 10, CartId = wc.Id, CartDeliveryGroupId = g.Id, Product2Id = p.Id, Type = 'Product');
    return CartExtension.CartTestUtil.getCart(wc.Id);
  }
  private static String run(List<String> domains) {
    try {
      CartExtension.PlaceOrderRequest req = new CartExtension.PlaceOrderRequest(mk());
      CartExtension.PlaceOrderResponse resp = new VP().validate(req, domains);
      return 'ok,respNull=' + (resp == null);
    } catch (Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
  }
  @IsTest static void domains() {
    List<String> out = new List<String>();
    out.add('pricing=' + run(new List<String>{'PRICING'}));
    out.add('taxes=' + run(new List<String>{'TAXES'}));
    out.add('all=' + run(new List<String>{'PRICING','PROMOTIONS','SHIPPING','TAXES'}));
    out.add('null=' + run(null));
    out.add('empty=' + run(new List<String>()));
    System.assert(false, 'R5|po|' + String.join(out, ' ## '));
  }
  @IsTest static void withMock() {
    Test.setMock(HttpCalloutMock.class, new M());
    System.assert(false, 'R5|pom|' + run(new List<String>{'PRICING'}));
  }
  public class M implements HttpCalloutMock {
    public HttpResponse respond(HttpRequest req) { HttpResponse r = new HttpResponse(); r.setStatusCode(200); r.setBody('{"status":"failure"}'); return r; }
  }
}
