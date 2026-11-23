try {
  CartExtension.Cart cart = CartExtension.CartTestUtil.createCart();
  System.debug('R5|noArg|name=' + cart.getName() + ';groups=' + cart.getCartDeliveryGroups().size() + ';items=' + cart.getCartItems().size());
} catch (Exception e) { System.debug('R5|noArg|' + e.getTypeName() + '|' + e.getMessage()); }
