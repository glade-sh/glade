@IsTest private class MixedDmlPartialShapesTest {
    @TestSetup static void seed() {
List<Account> customers=new List<Account>();
customers.add(new Account(Name='Shape delete_scalar'));
customers.add(new Account(Name='Shape delete_scalar'));
customers.add(new Account(Name='Shape insert_scalar'));
customers.add(new Account(Name='Shape insert_scalar'));
customers.add(new Account(Name='Shape update_scalar'));
customers.add(new Account(Name='Shape update_scalar'));
customers.add(new Account(Name='Shape upsert_insert_scalar'));
customers.add(new Account(Name='Shape upsert_insert_scalar'));
customers.add(new Account(Name='Shape upsert_update_scalar'));
customers.add(new Account(Name='Shape upsert_update_scalar'));
customers.add(new Account(Name='Shape undelete_scalar'));
customers.add(new Account(Name='Shape undelete_scalar'));
customers.add(new Account(Name='Shape merge_list'));
customers.add(new Account(Name='Shape merge_list'));
customers.add(new Account(Name='Shape insert_mixed_setup_first'));
customers.add(new Account(Name='Shape insert_mixed_setup_first'));
customers.add(new Account(Name='Shape insert_mixed_data_first'));
customers.add(new Account(Name='Shape insert_mixed_data_first'));
insert customers;
delete [SELECT Id FROM Account WHERE Name='Shape undelete_scalar'];
    }
    @IsTest static void observe_delete_scalar() { System.assert(false,'OBS|'+JSON.serialize(MixedDmlPartialShapes.observe('delete_scalar'))); }
    @IsTest static void observe_insert_scalar() { System.assert(false,'OBS|'+JSON.serialize(MixedDmlPartialShapes.observe('insert_scalar'))); }
    @IsTest static void observe_update_scalar() { System.assert(false,'OBS|'+JSON.serialize(MixedDmlPartialShapes.observe('update_scalar'))); }
    @IsTest static void observe_upsert_insert_scalar() { System.assert(false,'OBS|'+JSON.serialize(MixedDmlPartialShapes.observe('upsert_insert_scalar'))); }
    @IsTest static void observe_upsert_update_scalar() { System.assert(false,'OBS|'+JSON.serialize(MixedDmlPartialShapes.observe('upsert_update_scalar'))); }
    @IsTest static void observe_undelete_scalar() { System.assert(false,'OBS|'+JSON.serialize(MixedDmlPartialShapes.observe('undelete_scalar'))); }
    @IsTest static void observe_merge_list() { System.assert(false,'OBS|'+JSON.serialize(MixedDmlPartialShapes.observe('merge_list'))); }
    @IsTest static void observe_insert_mixed_setup_first() { System.assert(false,'OBS|'+JSON.serialize(MixedDmlPartialShapes.observe('insert_mixed_setup_first'))); }
    @IsTest static void observe_insert_mixed_data_first() { System.assert(false,'OBS|'+JSON.serialize(MixedDmlPartialShapes.observe('insert_mixed_data_first'))); }
}
