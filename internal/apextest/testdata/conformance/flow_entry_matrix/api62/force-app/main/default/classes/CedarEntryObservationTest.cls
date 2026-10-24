@IsTest private class CedarEntryObservationTest {
    @IsTest static void before_create_always() { System.assert(false, 'ENTRY_MATRIX|before_create_always|' + JSON.serialize(CedarEntryObserver.observe('before', 'create', 'always'))); }
    @IsTest static void before_update_always() { System.assert(false, 'ENTRY_MATRIX|before_update_always|' + JSON.serialize(CedarEntryObserver.observe('before', 'update', 'always'))); }
    @IsTest static void after_create_always() { System.assert(false, 'ENTRY_MATRIX|after_create_always|' + JSON.serialize(CedarEntryObserver.observe('after', 'create', 'always'))); }
    @IsTest static void after_update_always() { System.assert(false, 'ENTRY_MATRIX|after_update_always|' + JSON.serialize(CedarEntryObserver.observe('after', 'update', 'always'))); }
    @IsTest static void before_update_changed() { System.assert(false, 'ENTRY_MATRIX|before_update_changed|' + JSON.serialize(CedarEntryObserver.observe('before', 'update', 'changed'))); }
    @IsTest static void after_update_changed() { System.assert(false, 'ENTRY_MATRIX|after_update_changed|' + JSON.serialize(CedarEntryObserver.observe('after', 'update', 'changed'))); }
}
