@IsTest private class G32CommerceNativeTest {
@IsTest static void orchestrator_override(){System.assertEquals(true,G32CommerceOrchestratorOverride.observe());}
@IsTest static void inventory_override(){System.assertEquals(true,G32CommerceInventoryOverride.observe());}
@IsTest static void pricing_override(){System.assertEquals(true,G32CommercePricingOverride.observe());}
@IsTest static void promotions_override(){System.assertEquals(true,G32CommercePromotionsOverride.observe());}
@IsTest static void shipping_override(){System.assertEquals(true,G32CommerceShippingOverride.observe());}
@IsTest static void tax_override(){System.assertEquals(true,G32CommerceTaxOverride.observe());}
@IsTest static void split_override(){System.assertEquals(true,G32CommerceSplitOverride.observe());}
@IsTest static void split_mock_override(){System.assertEquals(true,G32CommerceSplitMockOverride.observe());}
@IsTest static void checkout_validate_override(){System.assertEquals(true,G32CommerceValidate.observe());}
@IsTest static void webstore_fields(){System.assertEquals(true,G32CommerceWebStore.observe());}
@IsTest static void webstore_null_boundary(){System.assertEquals(true,G32CommerceNullField.observe());}
}
