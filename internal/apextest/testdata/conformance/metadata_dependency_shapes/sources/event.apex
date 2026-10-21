Database.SaveResult result = EventBus.publish(new MeadowPulse__e(Note__c='sprout'));
System.debug('NATIVE|' + (String.valueOf(result.isSuccess())));
