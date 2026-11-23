@IsTest
private class R5ExecProbe {
  static Integer calls = 0;
  static String trace = '';
  private static CartExtension.Cart mk() {
    Account a = new Account(Name = 'acc'); insert a;
    WebStore ws = new WebStore(Name = 'ws', OptionsCartCalculateEnabled = true); insert ws;
    WebCart wc = new WebCart(Name = 'cart', WebStoreId = ws.Id, AccountId = a.Id, Status = 'Checkout'); insert wc;
    CartDeliveryGroup g = new CartDeliveryGroup(Name = 'grp', CartId = wc.Id); insert g;
    insert new CartItem(Name = 'ci1', CartId = wc.Id, CartDeliveryGroupId = g.Id, Quantity = 3, SKU = 'sku1', Type = 'Product');
    return CartExtension.CartTestUtil.getCart(wc.Id);
  }
  public class E extends CartExtension.CartCalculateExecutorMock {
    public E() {}
    public override void defaultInventory(CartExtension.CartCalculateCalculatorRequest request) { trace += 'dInv;'; }
    public override void defaultPrices(CartExtension.CartCalculateCalculatorRequest request) {
      trace += 'dPrices;';
      Iterator<CartExtension.CartItem> it = request.getCart().getCartItems().iterator();
      while (it.hasNext()) { it.next().setListPrice(9); }
    }
    public override void defaultShipping(CartExtension.CartCalculateCalculatorRequest request) { trace += 'dShip;'; }
    public override void defaultTax(CartExtension.CartCalculateCalculatorRequest request) { trace += 'dTax;'; }
    public override void defaultPromotions(CartExtension.CartCalculateCalculatorRequest request) { trace += 'dPromo;'; }
  }
  public class F extends CartExtension.CartCalculateExecutorMock {
    public F() {}
    public override void prices(CartExtension.CartCalculateCalculatorRequest request) { trace += 'prices;'; }
    public override void defaultPrices(CartExtension.CartCalculateCalculatorRequest request) { trace += 'dPricesF;'; }
  }
  public class PC extends CartExtension.PricingCartCalculator {
    public PC(CartExtension.CartCalculateExecutorMock m) { super(m); }
    public virtual override void calculate(CartExtension.CartCalculateCalculatorRequest request) { super.calculate(request); }
  }
  public class IC extends CartExtension.InventoryCartCalculator {
    public IC(CartExtension.CartCalculateExecutorMock m) { super(m); }
    public virtual override void calculate(CartExtension.CartCalculateCalculatorRequest request) { super.calculate(request); }
  }
  public class SC extends CartExtension.ShippingCartCalculator {
    public SC(CartExtension.CartCalculateExecutorMock m) { super(m); }
    public virtual override void calculate(CartExtension.CartCalculateCalculatorRequest request) { super.calculate(request); }
  }
  public class TC extends CartExtension.TaxCartCalculator {
    public TC(CartExtension.CartCalculateExecutorMock m) { super(m); }
    public virtual override void calculate(CartExtension.CartCalculateCalculatorRequest request) { super.calculate(request); }
  }
  public class PM extends CartExtension.PromotionsCartCalculator {
    public PM(CartExtension.CartCalculateExecutorMock m) { super(m); }
    public virtual override void calculate(CartExtension.CartCalculateCalculatorRequest request) { super.calculate(request); }
  }
  private static String run(String kind, CartExtension.CartCalculateExecutorMock m) {
    trace = '';
    try {
      CartExtension.Cart cart = mk();
      CartExtension.CartCalculateCalculatorRequest req = new CartExtension.CartCalculateCalculatorRequest(cart, CartExtension.OptionalBuyerActionDetails.empty());
      if (kind == 'pricing') new PC(m).calculate(req);
      else if (kind == 'inventory') new IC(m).calculate(req);
      else if (kind == 'shipping') new SC(m).calculate(req);
      else if (kind == 'tax') new TC(m).calculate(req);
      else if (kind == 'promotions') new PM(m).calculate(req);
      else if (kind == 'direct') m.prices(req);
      Decimal lp = cart.getCartItems().get(0).getListPrice();
      return kind + ':trace=' + trace + ',listPrice=' + lp;
    } catch (Exception e) { return kind + ':' + e.getTypeName() + '|' + e.getMessage() + ',trace=' + trace; }
  }
  @IsTest static void viaDefault() {
    List<String> out = new List<String>();
    for (String k : new List<String>{'pricing','inventory','shipping','tax','promotions','direct'}) out.add(run(k, new E()));
    System.assert(false, 'R5|via|' + String.join(out, ' ## '));
  }
  @IsTest static void viaOverride() {
    System.assert(false, 'R5|ovr|' + run('pricing', new F()) + ' ## ' + run('direct', new F()));
  }
}
