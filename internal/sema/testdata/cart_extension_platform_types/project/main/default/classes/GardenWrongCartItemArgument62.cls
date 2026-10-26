public class GardenWrongCartItemArgument62 {
    public static void callWrongType(CartExtension.CartItemList items, CartExtension.CartDeliveryGroup deliveryGroup) {
        items.add(deliveryGroup);
        items.remove(deliveryGroup);
    }
}
