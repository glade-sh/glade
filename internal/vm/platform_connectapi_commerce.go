package vm

import "fmt"

func (vm *VM) connectAPICommerceCartGetCartSummary(args []Value) (Value, error) {
	if len(args) < 3 || len(args) > 4 {
		return Null, fmt.Errorf("ConnectApi.CommerceCart.getCartSummary expects 3-4 arguments")
	}
	result := Object("ConnectApi.CartSummary")
	result.Fields["cartId"] = String("0cs000000000001")
	return result, nil
}

func (vm *VM) connectAPICommerceCartAddItemToCart(args []Value) (Value, error) {
	if len(args) < 4 || len(args) > 6 {
		return Null, fmt.Errorf("ConnectApi.CommerceCart.addItemToCart expects 4-6 arguments")
	}
	result := Object("ConnectApi.CartItem")
	result.Fields["cartItemId"] = String("0ci000000000001")
	return result, nil
}

func (vm *VM) connectAPICommerceCartAddItemsToCart(args []Value) (Value, error) {
	if len(args) < 3 || len(args) > 4 {
		return Null, fmt.Errorf("ConnectApi.CommerceCart.addItemsToCart expects 3-4 arguments")
	}
	results := make([]Value, 0)
	if len(args) >= 4 && args[3].Kind == ValueList {
		for i := range len(args[3].List) {
			batchResult := Object("ConnectApi.BatchResult")
			batchResult.Fields["id"] = String(fmt.Sprintf("0br00000000000%d", i+1))
			batchResult.Fields["batchedRecordId"] = String(fmt.Sprintf("0ci00000000000%d", i+1))
			batchResult.Fields["errors"] = typedList("List<ConnectApi.BatchError>")
			results = append(results, batchResult)
		}
	}
	return List(results...), nil
}

func (vm *VM) connectAPICommerceCartGetCartItems(args []Value) (Value, error) {
	if len(args) < 3 || len(args) > 11 {
		return Null, fmt.Errorf("ConnectApi.CommerceCart.getCartItems expects 3-11 arguments")
	}
	result := Object("ConnectApi.CartItemCollection")
	result.Fields["pageToken"] = Null
	result.Fields["total"] = Int(0)
	result.Fields["cartItems"] = typedList("List<ConnectApi.CartItem>")
	return result, nil
}

func (vm *VM) connectAPICommerceCatalogGetProduct(args []Value) (Value, error) {
	if len(args) < 3 || len(args) > 14 {
		return Null, fmt.Errorf("ConnectApi.CommerceCatalog.getProduct expects 3-14 arguments")
	}
	result := Object("ConnectApi.ProductDetail")
	result.Fields["productId"] = String("01t000000000001")
	result.Fields["productName"] = String("Local Product")
	return result, nil
}

func (vm *VM) connectAPICommerceStorePricingGetProductPrice(args []Value) (Value, error) {
	if len(args) < 3 || len(args) > 8 {
		return Null, fmt.Errorf("ConnectApi.CommerceStorePricing.getProductPrice expects 3-8 arguments")
	}
	result := Object("ConnectApi.ProductPrice")
	result.Fields["productId"] = String("01t000000000001")
	result.Fields["listPrice"] = Decimal(0.0)
	return result, nil
}

func (vm *VM) connectAPICommerceStorePricingGetProductPrices(args []Value) (Value, error) {
	if len(args) < 2 || len(args) > 4 {
		return Null, fmt.Errorf("ConnectApi.CommerceStorePricing.getProductPrices expects 2-4 arguments")
	}
	currency := "USD"
	if len(args) >= 4 {
		if supplied := scalarText(args[3]); supplied != "" {
			currency = supplied
		}
	}
	result := Object("ConnectApi.PricingResult")
	result.Fields["pricingBatches"] = typedList("List<ConnectApi.PricingBatchResult>")
	result.Fields["currencyIsoCode"] = String(currency)
	return result, nil
}
