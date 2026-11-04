@IsTest private class Case245ReflectionCaptureTest {
 @IsTest static void capture() { System.assert(false, 'OBSERVED|capture|'+JSON.serialize(Case245ReflectionCapture.observe())); }
}
