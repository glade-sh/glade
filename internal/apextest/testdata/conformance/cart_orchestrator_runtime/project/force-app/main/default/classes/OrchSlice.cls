global with sharing class OrchSlice extends CartExtension.CartCalculate {
 global OrchSlice() {}
 global OrchSlice(final CartExtension.CartCalculateExecutorMock executor) { super(executor); }
 global virtual override void calculate(CartExtension.CartCalculateOrchestratorRequest request) {
  CartExtension.Cart cart = request.getCart();
  CartExtension.BuyerActions ba = request.getBuyerActions();
  CartExtension.CartCalculateCalculatorRequest calc = new CartExtension.CartCalculateCalculatorRequest(cart, request.getOptionalBuyerActionDetails());
  Boolean checkout = CartExtension.CartStatusEnum.CHECKOUT == cart.getStatus();
  if (ba.isCartItemChanged() || ba.isRecalculationRequested()) { prices(calc); promotions(calc); }
  if (ba.isCheckoutStarted() || (ba.isRecalculationRequested() && checkout)) { inventory(calc); }
  if (ba.isEvaluateShippingRequested() || (ba.isDeliveryGroupChanged() && checkout)) { shipping(calc); shippingPromotions(calc); postShipping(calc); }
  if (ba.isEvaluateTaxesRequested() || (ba.isDeliveryMethodSelected() && checkout)) { taxes(calc); }
 }
}
