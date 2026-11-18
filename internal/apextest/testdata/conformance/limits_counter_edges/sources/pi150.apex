Edge_Evt__e e = new Edge_Evt__e(Note__c = 'x');
for (Integer i = 0; i < 150; i++) { EventBus.publish(e); }
obs = Limits.getPublishImmediateDML() + '|' + Limits.getLimitPublishImmediateDML();
