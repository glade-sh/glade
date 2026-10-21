@IsTest private class FeatureSettingsTest {
    @IsTest static void capture() {System.assert(false,'OBSERVED|capture|'+JSON.serialize(FeatureSettingsProbe.observe()));}
}
