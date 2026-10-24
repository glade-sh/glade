@IsTest private class ProfileInventoryTest {
    @IsTest static void inventory() { System.assert(false, 'CAPTURE|'+JSON.serialize(ProfileInventoryProbe.observe('inventory'))); }
    @IsTest static void missing() { System.assert(false, 'CAPTURE|'+JSON.serialize(ProfileInventoryProbe.observe('missing'))); }
    @IsTest static void limited() { System.assert(false, 'CAPTURE|'+JSON.serialize(ProfileInventoryProbe.observe('limited'))); }
    @IsTest static void readOnly() { System.assert(false, 'CAPTURE|'+JSON.serialize(ProfileInventoryProbe.observe('readOnly'))); }
}
