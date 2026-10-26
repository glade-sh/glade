CartExtension.CartItemList items = new CartExtension.CartItemList();
for (CartExtension.CartItem item : items) {
    System.debug('FOR_ITEM|' + item.getName());
}
System.debug('FOR_DONE');
