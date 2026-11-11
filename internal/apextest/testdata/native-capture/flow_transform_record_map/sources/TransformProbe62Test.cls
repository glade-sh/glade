@IsTest private class TransformProbe62Test {
 @IsTest static void observe() {
  Flow.Interview f = Flow.Interview.createInterview('TransformProbe', new Map<String,Object>{'InputText' => 'Map sample'}); f.start();
  Contact c = (Contact)f.getVariableValue('OutputRecord');
  System.assertEquals('Map sample', c.LastName); System.assertEquals(null, c.Title);
  System.assertEquals(true, c.getPopulatedFieldsAsMap().containsKey('Title'));
 }
}
