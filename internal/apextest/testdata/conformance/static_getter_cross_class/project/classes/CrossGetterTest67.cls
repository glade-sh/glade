@IsTest private class CrossGetterTest67 {
 @IsTest static void crossSetter() { System.assertEquals('ok',CrossValue67.field); System.assertEquals(2,CrossValue67.visits); }
 @IsTest static void crossReadOnly() { System.assertEquals('ok',CrossValue67.readOnly); System.assertEquals(2,CrossValue67.visits); }
 @IsTest static void directAccess() { System.assertEquals('ok',CrossValue67.direct); System.assertEquals(1,CrossValue67.visits); }
 @IsTest static void recursiveCycle() { String currentValue=CrossValue67.recursiveValue; }
}
