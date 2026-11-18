List<Tb_Evt__e> evs = new List<Tb_Evt__e>();
for (Integer i = 0; i < 500; i++) { evs.add(new Tb_Evt__e(Note__c = 'e' + i)); }
EventBus.publish(evs);
Test.startTest();
List<Database.SaveResult> rs = EventBus.publish(new List<Tb_Evt__e>{ new Tb_Evt__e(Note__c = 'x') });
String a = '' + rs[0].isSuccess() + (rs[0].isSuccess() ? '' : ':' + rs[0].getErrors()[0].getMessage());
Test.stopTest();
List<Database.SaveResult> rs2 = EventBus.publish(new List<Tb_Evt__e>{ new Tb_Evt__e(Note__c = 'y') });
obs = a + '|' + rs2[0].isSuccess() + (rs2[0].isSuccess() ? '' : ':' + rs2[0].getErrors()[0].getMessage());
