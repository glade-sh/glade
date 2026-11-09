@isTest
private class C4EvtTest {
    @isTest static void pe_npe_deliver() {
        Map<String, Object> o = new Map<String, Object>();
        
        EventBus.publish(new C4Evt__e(Note__c = 'npe'));
        Integer q = Limits.getQueries();
        try { Test.getEventBus().deliver(); o.put('deliver', 'ok'); } catch (Exception e) { o.put('deliver', e.getTypeName() + '|' + e.getMessage()); }
        o.put('testQueriesBefore', q);
        o.put('next', 'reached');
        EventBus.publish(new C4Evt__e(Note__c = 'none'));
        try { Test.getEventBus().deliver(); o.put('deliver2', 'ok'); } catch (Exception e) { o.put('deliver2', e.getTypeName() + '|' + e.getMessage()); }
        o.put('rows', C4Probe.dump());
        System.assert(false, 'OBS|' + JSON.serialize(o));
    }
    @isTest static void pe_npe_stop() {
        Map<String, Object> o = new Map<String, Object>();
        
        Test.startTest();
        EventBus.publish(new C4Evt__e(Note__c = 'npe'));
        try { Test.stopTest(); o.put('stop', 'ok'); } catch (Exception e) { o.put('stop', e.getTypeName() + '|' + e.getMessage()); }
        o.put('next', 'reached');
        o.put('rows', C4Probe.dump());
        System.assert(false, 'OBS|' + JSON.serialize(o));
    }
    @isTest static void pe_limit_deliver() {
        Map<String, Object> o = new Map<String, Object>();
        
        EventBus.publish(new C4Evt__e(Note__c = 'limit'));
        Integer q = Limits.getQueries();
        try { Test.getEventBus().deliver(); o.put('deliver', 'ok'); } catch (Exception e) { o.put('deliver', e.getTypeName() + '|' + e.getMessage()); }
        o.put('testQueriesBefore', q);
        o.put('next', 'reached');
        EventBus.publish(new C4Evt__e(Note__c = 'none'));
        try { Test.getEventBus().deliver(); o.put('deliver2', 'ok'); } catch (Exception e) { o.put('deliver2', e.getTypeName() + '|' + e.getMessage()); }
        o.put('rows', C4Probe.dump());
        System.assert(false, 'OBS|' + JSON.serialize(o));
    }
    @isTest static void pe_limit_stop() {
        Map<String, Object> o = new Map<String, Object>();
        
        Test.startTest();
        EventBus.publish(new C4Evt__e(Note__c = 'limit'));
        try { Test.stopTest(); o.put('stop', 'ok'); } catch (Exception e) { o.put('stop', e.getTypeName() + '|' + e.getMessage()); }
        o.put('next', 'reached');
        o.put('rows', C4Probe.dump());
        System.assert(false, 'OBS|' + JSON.serialize(o));
    }
    @isTest static void pe_dmlex_deliver() {
        Map<String, Object> o = new Map<String, Object>();
        
        EventBus.publish(new C4Evt__e(Note__c = 'dmlex'));
        Integer q = Limits.getQueries();
        try { Test.getEventBus().deliver(); o.put('deliver', 'ok'); } catch (Exception e) { o.put('deliver', e.getTypeName() + '|' + e.getMessage()); }
        o.put('testQueriesBefore', q);
        o.put('next', 'reached');
        EventBus.publish(new C4Evt__e(Note__c = 'none'));
        try { Test.getEventBus().deliver(); o.put('deliver2', 'ok'); } catch (Exception e) { o.put('deliver2', e.getTypeName() + '|' + e.getMessage()); }
        o.put('rows', C4Probe.dump());
        System.assert(false, 'OBS|' + JSON.serialize(o));
    }
    @isTest static void pe_dmlex_stop() {
        Map<String, Object> o = new Map<String, Object>();
        
        Test.startTest();
        EventBus.publish(new C4Evt__e(Note__c = 'dmlex'));
        try { Test.stopTest(); o.put('stop', 'ok'); } catch (Exception e) { o.put('stop', e.getTypeName() + '|' + e.getMessage()); }
        o.put('next', 'reached');
        o.put('rows', C4Probe.dump());
        System.assert(false, 'OBS|' + JSON.serialize(o));
    }
    @isTest static void pe_listx_deliver() {
        Map<String, Object> o = new Map<String, Object>();
        
        EventBus.publish(new C4Evt__e(Note__c = 'listx'));
        Integer q = Limits.getQueries();
        try { Test.getEventBus().deliver(); o.put('deliver', 'ok'); } catch (Exception e) { o.put('deliver', e.getTypeName() + '|' + e.getMessage()); }
        o.put('testQueriesBefore', q);
        o.put('next', 'reached');
        EventBus.publish(new C4Evt__e(Note__c = 'none'));
        try { Test.getEventBus().deliver(); o.put('deliver2', 'ok'); } catch (Exception e) { o.put('deliver2', e.getTypeName() + '|' + e.getMessage()); }
        o.put('rows', C4Probe.dump());
        System.assert(false, 'OBS|' + JSON.serialize(o));
    }
    @isTest static void pe_listx_stop() {
        Map<String, Object> o = new Map<String, Object>();
        
        Test.startTest();
        EventBus.publish(new C4Evt__e(Note__c = 'listx'));
        try { Test.stopTest(); o.put('stop', 'ok'); } catch (Exception e) { o.put('stop', e.getTypeName() + '|' + e.getMessage()); }
        o.put('next', 'reached');
        o.put('rows', C4Probe.dump());
        System.assert(false, 'OBS|' + JSON.serialize(o));
    }
    @isTest static void cdc_npe_deliver() {
        Map<String, Object> o = new Map<String, Object>();
        Test.enableChangeDataCapture();
        insert new C4Cdc__c(Name = 'npe');
        Integer q = Limits.getQueries();
        try { Test.getEventBus().deliver(); o.put('deliver', 'ok'); } catch (Exception e) { o.put('deliver', e.getTypeName() + '|' + e.getMessage()); }
        o.put('testQueriesBefore', q);
        o.put('next', 'reached');
        EventBus.publish(new C4Evt__e(Note__c = 'none'));
        try { Test.getEventBus().deliver(); o.put('deliver2', 'ok'); } catch (Exception e) { o.put('deliver2', e.getTypeName() + '|' + e.getMessage()); }
        o.put('rows', C4Probe.dump());
        System.assert(false, 'OBS|' + JSON.serialize(o));
    }
    @isTest static void cdc_npe_stop() {
        Map<String, Object> o = new Map<String, Object>();
        Test.enableChangeDataCapture();
        Test.startTest();
        insert new C4Cdc__c(Name = 'npe');
        try { Test.stopTest(); o.put('stop', 'ok'); } catch (Exception e) { o.put('stop', e.getTypeName() + '|' + e.getMessage()); }
        o.put('next', 'reached');
        o.put('rows', C4Probe.dump());
        System.assert(false, 'OBS|' + JSON.serialize(o));
    }
    @isTest static void cdc_limit_deliver() {
        Map<String, Object> o = new Map<String, Object>();
        Test.enableChangeDataCapture();
        insert new C4Cdc__c(Name = 'limit');
        Integer q = Limits.getQueries();
        try { Test.getEventBus().deliver(); o.put('deliver', 'ok'); } catch (Exception e) { o.put('deliver', e.getTypeName() + '|' + e.getMessage()); }
        o.put('testQueriesBefore', q);
        o.put('next', 'reached');
        EventBus.publish(new C4Evt__e(Note__c = 'none'));
        try { Test.getEventBus().deliver(); o.put('deliver2', 'ok'); } catch (Exception e) { o.put('deliver2', e.getTypeName() + '|' + e.getMessage()); }
        o.put('rows', C4Probe.dump());
        System.assert(false, 'OBS|' + JSON.serialize(o));
    }
    @isTest static void cdc_limit_stop() {
        Map<String, Object> o = new Map<String, Object>();
        Test.enableChangeDataCapture();
        Test.startTest();
        insert new C4Cdc__c(Name = 'limit');
        try { Test.stopTest(); o.put('stop', 'ok'); } catch (Exception e) { o.put('stop', e.getTypeName() + '|' + e.getMessage()); }
        o.put('next', 'reached');
        o.put('rows', C4Probe.dump());
        System.assert(false, 'OBS|' + JSON.serialize(o));
    }
    @isTest static void cdc_dmlex_deliver() {
        Map<String, Object> o = new Map<String, Object>();
        Test.enableChangeDataCapture();
        insert new C4Cdc__c(Name = 'dmlex');
        Integer q = Limits.getQueries();
        try { Test.getEventBus().deliver(); o.put('deliver', 'ok'); } catch (Exception e) { o.put('deliver', e.getTypeName() + '|' + e.getMessage()); }
        o.put('testQueriesBefore', q);
        o.put('next', 'reached');
        EventBus.publish(new C4Evt__e(Note__c = 'none'));
        try { Test.getEventBus().deliver(); o.put('deliver2', 'ok'); } catch (Exception e) { o.put('deliver2', e.getTypeName() + '|' + e.getMessage()); }
        o.put('rows', C4Probe.dump());
        System.assert(false, 'OBS|' + JSON.serialize(o));
    }
    @isTest static void cdc_dmlex_stop() {
        Map<String, Object> o = new Map<String, Object>();
        Test.enableChangeDataCapture();
        Test.startTest();
        insert new C4Cdc__c(Name = 'dmlex');
        try { Test.stopTest(); o.put('stop', 'ok'); } catch (Exception e) { o.put('stop', e.getTypeName() + '|' + e.getMessage()); }
        o.put('next', 'reached');
        o.put('rows', C4Probe.dump());
        System.assert(false, 'OBS|' + JSON.serialize(o));
    }
    @isTest static void cdc_listx_deliver() {
        Map<String, Object> o = new Map<String, Object>();
        Test.enableChangeDataCapture();
        insert new C4Cdc__c(Name = 'listx');
        Integer q = Limits.getQueries();
        try { Test.getEventBus().deliver(); o.put('deliver', 'ok'); } catch (Exception e) { o.put('deliver', e.getTypeName() + '|' + e.getMessage()); }
        o.put('testQueriesBefore', q);
        o.put('next', 'reached');
        EventBus.publish(new C4Evt__e(Note__c = 'none'));
        try { Test.getEventBus().deliver(); o.put('deliver2', 'ok'); } catch (Exception e) { o.put('deliver2', e.getTypeName() + '|' + e.getMessage()); }
        o.put('rows', C4Probe.dump());
        System.assert(false, 'OBS|' + JSON.serialize(o));
    }
    @isTest static void cdc_listx_stop() {
        Map<String, Object> o = new Map<String, Object>();
        Test.enableChangeDataCapture();
        Test.startTest();
        insert new C4Cdc__c(Name = 'listx');
        try { Test.stopTest(); o.put('stop', 'ok'); } catch (Exception e) { o.put('stop', e.getTypeName() + '|' + e.getMessage()); }
        o.put('next', 'reached');
        o.put('rows', C4Probe.dump());
        System.assert(false, 'OBS|' + JSON.serialize(o));
    }
}
