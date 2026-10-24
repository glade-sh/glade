public class GardenCartIteration67 {
    public static Integer count(CartExtension.CartItemList items) {
        Integer countItems = 0;
        for (CartExtension.CartItem item : items) {
            countItems++;
        }
        return countItems;
    }
}
