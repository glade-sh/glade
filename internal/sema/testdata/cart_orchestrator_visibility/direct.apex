CartExtension.CartCalculate value = null;
try {
    value.shippingPromotions((CartExtension.CartCalculateCalculatorRequest)null);
    System.assert(false, 'expected null receiver');
} catch (NullPointerException expected) {
    System.assert(true);
}
