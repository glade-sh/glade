Database.SaveResult published = EventBus.publish(new BoundaryDirect__e(Key__c='Delivery anonymous'));
System.debug('OBS|'+JSON.serialize(new Map<String,Object>{'published'=>published.isSuccess()}));
