@IsTest private class GardenSurfaceTest {
@IsTest static void orchestrator_override() { System.assertEquals(true, GardenOrchestratorOverride.observe()); }
@IsTest static void pricing_override() { System.assertEquals(true, GardenPricingOverride.observe()); }
@IsTest static void promotions_override() { System.assertEquals(true, GardenPromotionsOverride.observe()); }
@IsTest static void shipping_override() { System.assertEquals(true, GardenShippingOverride.observe()); }
@IsTest static void tax_override() { System.assertEquals(true, GardenTaxOverride.observe()); }
@IsTest static void split_override() { System.assertEquals(true, GardenSplitOverride.observe()); }
@IsTest static void split_mock_override() { System.assertEquals(true, GardenSplitMockOverride.observe()); }
}
