@IsTest global class OrchSliceTest {
 global class Exec extends CartExtension.CartCalculateExecutorMock {
  global Exec() {}
  private void tag(CartExtension.CartCalculateCalculatorRequest r, String t) { CartExtension.Cart c = r.getCart(); c.setName(c.getName() + ',' + t); }
  global override void prices(CartExtension.CartCalculateCalculatorRequest r) { tag(r, 'P'); }
  global override void promotions(CartExtension.CartCalculateCalculatorRequest r) { tag(r, 'M'); }
  global override void inventory(CartExtension.CartCalculateCalculatorRequest r) { tag(r, 'I'); }
  global override void shipping(CartExtension.CartCalculateCalculatorRequest r) { tag(r, 'S'); }
  global override void shippingPromotions(CartExtension.CartCalculateCalculatorRequest r) { tag(r, 'SP'); }
  global override void tax(CartExtension.CartCalculateCalculatorRequest r) { tag(r, 'T'); }
  global override void postShipping(CartExtension.CartCalculateCalculatorRequest r) { tag(r, 'PS'); }
 }
 static Exec executor = new Exec();
 static OrchSlice orchestrator = new OrchSlice(executor);
 private static CartExtension.Cart arrange(String status) {
  Account a = new Account(Name='My Account'); insert a;
  WebStore ws = new WebStore(Name='My WebStore'); insert ws;
  WebCart wc = new WebCart(Name='Cart', WebStoreId=ws.Id, AccountId=a.Id, Status=status); insert wc;
  CartDeliveryGroup dg = new CartDeliveryGroup(Name='G', CartId=wc.Id); insert dg;
  CartItem ci = new CartItem(Name='I', CartId=wc.Id, CartDeliveryGroupId=dg.Id); insert ci;
  return CartExtension.CartTestUtil.getCart(wc.Id);
 }
 private static void run(String method, String status, Set<String> flags) {
  Map<String,Object> o = new Map<String,Object>();
  try {
   CartExtension.Cart cart = arrange(status);
   CartExtension.BuyerActionsMock m = new CartExtension.BuyerActionsMock(cart);
   if (flags.contains('item')) m.setCartItemChanged(true);
   if (flags.contains('checkout')) m.setCheckoutStarted(true);
   if (flags.contains('recalc')) m.setRecalculationRequested(true);
   if (flags.contains('ship')) m.setEvaluateShippingRequested(true);
   if (flags.contains('group')) m.setDeliveryGroupChanged(true);
   if (flags.contains('method')) m.setDeliveryMethodSelected(true);
   if (flags.contains('tax')) m.setEvaluateTaxesRequested(true);
   Test.startTest();
   orchestrator.calculate(new CartExtension.CartCalculateOrchestratorRequest(cart, m, CartExtension.OptionalBuyerActionDetails.empty()));
   Test.stopTest();
   o.put('name', cart.getName()); o.put('status', String.valueOf(cart.getStatus()));
  } catch (Exception e) { o.put('exception', e.getTypeName()); o.put('message', e.getMessage()); }
  System.assert(false, 'OBSERVED|' + method + '|' + JSON.serialize(o));
 }
 @IsTest static void add_to_cart() { run('add_to_cart', 'Active', new Set<String>{'item'}); }
 @IsTest static void start_checkout() { run('start_checkout', 'Active', new Set<String>{'checkout'}); }
 @IsTest static void checkout_recalc() { run('checkout_recalc', 'Checkout', new Set<String>{'recalc'}); }
 @IsTest static void shipping_and_taxes() { run('shipping_and_taxes', 'Checkout', new Set<String>{'group', 'method', 'tax', 'ship'}); }
 @IsTest static void no_flags() { run('no_flags', 'Active', new Set<String>()); }
}
