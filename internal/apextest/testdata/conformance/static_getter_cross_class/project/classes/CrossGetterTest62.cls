@IsTest private class CrossGetterTest62 {
 @IsTest static void crossSetter() { System.assertEquals('ok',CrossValue62.field); System.assertEquals(2,CrossValue62.visits); }
 @IsTest static void crossReadOnly() { System.assertEquals('ok',CrossValue62.readOnly); System.assertEquals(2,CrossValue62.visits); }
 @IsTest static void directAccess() { System.assertEquals('ok',CrossValue62.direct); System.assertEquals(1,CrossValue62.visits); }
 @IsTest static void recursiveCycle() { String currentValue=CrossValue62.recursiveValue; }
}
