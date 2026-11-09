@IsTest private class CartListSlice {
 private static CartExtension.Cart arrange() {
  Account a = new Account(Name='My Account'); insert a;
  WebStore ws = new WebStore(Name='My WebStore'); insert ws;
  WebCart wc = new WebCart(Name='My Cart', WebStoreId=ws.Id, AccountId=a.Id); insert wc;
  CartDeliveryGroup dg = new CartDeliveryGroup(Name='My Delivery Group', CartId=wc.Id); insert dg;
  CartItem ci = new CartItem(Name='My Cart Item', CartId=wc.Id, CartDeliveryGroupId=dg.Id); insert ci;
  WebCartAdjustmentBasis b = new WebCartAdjustmentBasis(Name='My Coupon', WebCartId=wc.Id); insert b;
  return CartExtension.CartTestUtil.getCart(wc.Id);
 }
 private static void done(String method, Map<String,Object> observed) { System.assert(false, 'OBSERVED|' + method + '|' + JSON.serialize(observed)); }
 @IsTest static void loaded_lists() {
  Map<String,Object> o = new Map<String,Object>();
  try {
   CartExtension.Cart cart = arrange();
   CartExtension.CartItemList items = cart.getCartItems();
   o.put('itemsSize', items.size()); o.put('itemsEmpty', items.isEmpty()); o.put('item0', items.get(0).getName()); o.put('item0Type', String.valueOf(items.get(0).getType()));
   CartExtension.CartDeliveryGroupList groups = cart.getCartDeliveryGroups();
   o.put('groupsSize', groups.size()); o.put('group0', groups.get(0).getName());
   Iterator<CartExtension.CartDeliveryGroup> it = groups.iterator();
   o.put('groupHasNext', it.hasNext()); o.put('groupNext', it.next().getName()); o.put('groupHasNextAfter', it.hasNext());
   CartExtension.CartAdjustmentBasisList bases = cart.getCartAdjustmentBases();
   o.put('basesSize', bases.size()); o.put('base0Present', bases.get(0) != null);
   o.put('outputsSize', cart.getCartValidationOutputs().size()); o.put('outputsEmpty', cart.getCartValidationOutputs().isEmpty());
   o.put('sameListObject', cart.getCartItems() === items);
  } catch (Exception e) { o.put('exception', e.getTypeName()); o.put('message', e.getMessage()); }
  done('loaded_lists', o);
 }
 @IsTest static void list_bounds() {
  Map<String,Object> o = new Map<String,Object>();
  CartExtension.Cart cart = arrange();
  try { cart.getCartItems().get(5); } catch (Exception e) { o.put('get5', e.getTypeName() + ': ' + e.getMessage()); }
  try { cart.getCartItems().get(-1); } catch (Exception e) { o.put('getMinus1', e.getTypeName() + ': ' + e.getMessage()); }
  try { cart.getCartValidationOutputs().get(0); } catch (Exception e) { o.put('outputsGet0', e.getTypeName() + ': ' + e.getMessage()); }
  Iterator<CartExtension.CartValidationOutput> it = cart.getCartValidationOutputs().iterator();
  o.put('emptyHasNext', it.hasNext());
  try { it.next(); } catch (Exception e) { o.put('emptyNext', e.getTypeName() + ': ' + e.getMessage()); }
  done('list_bounds', o);
 }
 @IsTest static void list_mutation() {
  Map<String,Object> o = new Map<String,Object>();
  try {
   CartExtension.Cart cart = arrange();
   CartExtension.CartValidationOutputList outputs = cart.getCartValidationOutputs();
   CartExtension.CartValidationOutput first = new CartExtension.CartValidationOutput(CartExtension.CartValidationOutputTypeEnum.TAXES, CartExtension.CartValidationOutputLevelEnum.ERROR);
   CartExtension.CartValidationOutput second = new CartExtension.CartValidationOutput(CartExtension.CartValidationOutputTypeEnum.PRICING, CartExtension.CartValidationOutputLevelEnum.INFO);
   outputs.add(first); outputs.add(second);
   o.put('sizeAfterAdd', outputs.size());
   o.put('seenByNextAccessor', cart.getCartValidationOutputs().size());
   CartExtension.CartValidationOutput viaIterator = outputs.iterator().next();
   o.put('iteratorFirstType', String.valueOf(viaIterator.getType())); o.put('iteratorFirstLevel', String.valueOf(viaIterator.getLevel()));
   o.put('indexOfSecond', outputs.indexOf(second));
   outputs.remove(first);
   o.put('sizeAfterRemove', outputs.size()); o.put('remainingType', String.valueOf(outputs.get(0).getType()));
   outputs.clear();
   o.put('sizeAfterClear', outputs.size());
   CartExtension.CartItemList items = cart.getCartItems();
   CartExtension.CartDeliveryGroup grp = cart.getCartDeliveryGroups().get(0);
   items.add(new CartExtension.CartItem(CartExtension.SalesItemTypeEnum.PRODUCT, grp, 'Garden item'));
   o.put('itemsAfterAdd', items.size()); o.put('addedName', items.get(1).getName()); o.put('storedItems', [SELECT COUNT() FROM CartItem WHERE CartId = :cart.getId() WITH SYSTEM_MODE]);
  } catch (Exception e) { o.put('exception', e.getTypeName()); o.put('message', e.getMessage()); }
  done('list_mutation', o);
 }
 @IsTest static void buyer_actions() {
  Map<String,Object> o = new Map<String,Object>();
  try {
   CartExtension.Cart cart = arrange();
   CartExtension.BuyerActionsMock mock = new CartExtension.BuyerActionsMock(cart);
   CartExtension.BuyerActions actions = mock;
   List<Boolean> before = new List<Boolean>{actions.isCartItemChanged(), actions.isCheckoutStarted(), actions.isCouponChanged(), actions.isDeliveryGroupChanged(), actions.isDeliveryMethodSelected(), actions.isEvaluateShippingRequested(), actions.isEvaluateTaxesRequested(), actions.isRecalculationRequested()};
   o.put('before', before);
   mock.setCartItemChanged(true); mock.setCheckoutStarted(true); mock.setCouponChanged(true); mock.setDeliveryGroupChanged(true); mock.setDeliveryMethodSelected(true); mock.setEvaluateShippingRequested(true); mock.setEvaluateTaxesRequested(true); mock.setRecalculationRequested(true);
   o.put('after', new List<Boolean>{actions.isCartItemChanged(), actions.isCheckoutStarted(), actions.isCouponChanged(), actions.isDeliveryGroupChanged(), actions.isDeliveryMethodSelected(), actions.isEvaluateShippingRequested(), actions.isEvaluateTaxesRequested(), actions.isRecalculationRequested()});
   mock.setCartItemChanged(false);
   o.put('reset', actions.isCartItemChanged());
   CartExtension.CartCalculateOrchestratorRequest request = new CartExtension.CartCalculateOrchestratorRequest(cart, mock, CartExtension.OptionalBuyerActionDetails.empty());
   o.put('requestCouponChanged', request.getBuyerActions().isCouponChanged());
  } catch (Exception e) { o.put('exception', e.getTypeName()); o.put('message', e.getMessage()); }
  done('buyer_actions', o);
 }
}
