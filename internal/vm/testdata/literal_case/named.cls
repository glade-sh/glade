@IsTest private class CaseProbeTest {
 @IsTest static void preservesLiteralBytes() {
  System.assertEquals('4162432078597a', EncodingUtil.convertToHex(Blob.valueOf('AbC xYz')));
  System.assertEquals('', EncodingUtil.convertToHex(Blob.valueOf('')));
  System.assertEquals('cea94d6978', EncodingUtil.convertToHex(Blob.valueOf('ΩMix')));
  System.assertEquals('4127425c43', EncodingUtil.convertToHex(Blob.valueOf('A\'B\\C')));
 }
}
