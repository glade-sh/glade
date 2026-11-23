@IsTest
private class R5PoDeep {
  public virtual class Mid extends CartExtension.CheckoutPlaceOrder {}
  public class Deep extends Mid {
    public virtual override CartExtension.PlaceOrderResponse validate(CartExtension.PlaceOrderRequest r, List<String> d) { return super.validate(r, d); }
  }
  public virtual class Mid2 extends Mid {}
  public class Deep2 extends Mid2 {
    public virtual override CartExtension.PlaceOrderResponse validate(CartExtension.PlaceOrderRequest r, List<String> d) { return super.validate(r, d); }
  }
  @IsTest static void deep() {
    Account a = new Account(Name = 'acc'); insert a;
    WebStore ws = new WebStore(Name = 'ws'); insert ws;
    WebCart wc = new WebCart(Name = 'cart', WebStoreId = ws.Id, AccountId = a.Id); insert wc;
    CartExtension.PlaceOrderRequest req = new CartExtension.PlaceOrderRequest(CartExtension.CartTestUtil.getCart(wc.Id));
    String msg = '';
    try { new Deep().validate(req, new List<String>{'PRICING'}); msg += 'deep=ok'; } catch (Exception e) { msg += 'deep=' + e.getTypeName() + '|' + e.getMessage(); }
    try { new Deep2().validate(req, new List<String>{'PRICING'}); msg += ',deep3=ok'; } catch (Exception e) { msg += ',deep3=' + e.getTypeName() + '|' + e.getMessage(); }
    try { new Mid().validate(req, new List<String>{'PRICING'}); msg += ',mid=ok'; } catch (Exception e) { msg += ',mid=' + e.getTypeName() + '|' + e.getMessage(); }
    System.assert(false, 'R5|deep|' + msg);
  }
}
