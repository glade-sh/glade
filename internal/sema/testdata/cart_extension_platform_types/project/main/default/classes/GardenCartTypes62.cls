@IsTest private class GardenCartTypes62 {
    @IsTest static void observeSurface() {
        List<CartExtension.SalesItemTypeEnum> itemTypes = CartExtension.SalesItemTypeEnum.values();
        List<String> names = new List<String>();
        for (CartExtension.SalesItemTypeEnum itemType : itemTypes) {
            names.add(String.valueOf(itemType));
        }

        CartExtension.Cart cart = CartExtension.CartTestUtil.createCart(CartExtension.WebStoreTypeEnum.B2CE);
        CartExtension.CartDeliveryGroupList groups = cart.getCartDeliveryGroups();
        CartExtension.CartDeliveryGroup deliveryGroup;
        if (groups.isEmpty()) {
            deliveryGroup = new CartExtension.CartDeliveryGroup();
            groups.add(deliveryGroup);
        } else {
            deliveryGroup = groups.get(0);
        }
        CartExtension.CartItem item = new CartExtension.CartItem(CartExtension.SalesItemTypeEnum.PRODUCT, deliveryGroup, 'Garden item');
        CartExtension.CartItemList items = cart.getCartItems();
        Map<String, Object> observed = new Map<String, Object>();
        observed.put('enumValues', names);
        observed.put('productOrdinal', CartExtension.SalesItemTypeEnum.valueOf('PRODUCT').ordinal());
        observed.put('chargeOrdinal', CartExtension.SalesItemTypeEnum.valueOf('CHARGE').ordinal());
        observed.put('itemType', String.valueOf(item.getType()));
        observed.put('itemName', item.getName());
        observed.put('emptyBefore', items.isEmpty());
        observed.put('sizeBefore', items.size());

        items.add(item);
        observed.put('sizeAfterAdd', items.size());
        observed.put('sameItem', items.get(0) == item);
        observed.put('indexOfItem', items.indexOf(item));
        Iterator<CartExtension.CartItem> itemIterator = items.iterator();
        List<String> iteratorNames = new List<String>();
        observed.put('iteratorHasNext', itemIterator.hasNext());
        iteratorNames.add(itemIterator.next().getName());
        iteratorNames.add(itemIterator.next().getName());
        observed.put('iteratorItemNames', iteratorNames);
        observed.put('iteratorHasNextAfterLast', itemIterator.hasNext());

        items.remove(item);
        observed.put('sizeAfterRemove', items.size());
        observed.put('emptyAfterRemove', items.isEmpty());
        items.add(item);
        items.clear();
        observed.put('sizeAfterClear', items.size());
        observed.put('emptyAfterClear', items.isEmpty());
        System.assert(false, 'OBSERVED|cart_extension_platform_types|' + JSON.serialize(observed));
    }
}
