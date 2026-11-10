@IsTest private class TextDefaultMatrixTest67 {
 @IsTest static void observe() {
  Map<String,Object> observed=(Map<String,Object>)JSON.deserializeUntyped(TextDefaultMatrix67.run());
  System.assertEquals(null, ((Map<String,Object>)observed.get('Combined')).get('value'), 'Combined');
  System.assertEquals('\'a\' & \'b\'', ((Map<String,Object>)observed.get('Combined')).get('formula'), 'Combined formula');
  System.assertEquals(null, ((Map<String,Object>)observed.get('First')).get('value'), 'First');
  System.assertEquals('\'Alpha\'', ((Map<String,Object>)observed.get('First')).get('formula'), 'First formula');
  System.assertEquals(null, ((Map<String,Object>)observed.get('NumericArithmetic')).get('value'), 'NumericArithmetic');
  System.assertEquals('2 + 3', ((Map<String,Object>)observed.get('NumericArithmetic')).get('formula'), 'NumericArithmetic formula');
  System.assertEquals(null, ((Map<String,Object>)observed.get('NumericFunction')).get('value'), 'NumericFunction');
  System.assertEquals('ABS(-4)', ((Map<String,Object>)observed.get('NumericFunction')).get('formula'), 'NumericFunction formula');
  System.assertEquals(null, ((Map<String,Object>)observed.get('Quoted')).get('value'), 'Quoted');
  System.assertEquals('\'O\\\'Brien\'', ((Map<String,Object>)observed.get('Quoted')).get('formula'), 'Quoted formula');
  System.assertEquals(null, ((Map<String,Object>)observed.get('Second')).get('value'), 'Second');
  System.assertEquals('\'Two words\'', ((Map<String,Object>)observed.get('Second')).get('formula'), 'Second formula');
  System.assertEquals(null, ((Map<String,Object>)observed.get('TextConcat')).get('value'), 'TextConcat');
  System.assertEquals('\'left\' & \'right\'', ((Map<String,Object>)observed.get('TextConcat')).get('formula'), 'TextConcat formula');
  System.assertEquals(null, ((Map<String,Object>)observed.get('TextFunction')).get('value'), 'TextFunction');
  System.assertEquals('UPPER(\'mixed\')', ((Map<String,Object>)observed.get('TextFunction')).get('formula'), 'TextFunction formula');
  System.assertEquals(null, ((Map<String,Object>)observed.get('TextPlus')).get('value'), 'TextPlus');
  System.assertEquals('\'a\' + \'b\'', ((Map<String,Object>)observed.get('TextPlus')).get('formula'), 'TextPlus formula');
 }
}
