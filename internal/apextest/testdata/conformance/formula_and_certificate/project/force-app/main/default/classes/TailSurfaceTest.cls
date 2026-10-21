@IsTest private class TailSurfaceTest {
 @IsTest static void ordinalFormula() {
  Map<String,Object> result=new Map<String,Object>();
  try {
   FormulaEval.FormulaInstance formula=Formula.builder().withType(Account.class).withReturnType(FormulaEval.FormulaReturnType.DECIMAL).withFormula('DAYOFYEAR(DATE(2024,2,29))').build();
   result.put('value',formula.evaluate(new Account()));
  } catch(Exception ex){result.put('type',ex.getTypeName());result.put('message',ex.getMessage());}
  System.assert(false,'OBSERVED|ordinalFormula|'+JSON.serialize(result));
 }
 @IsTest static void missingCertificate() {
  Map<String,Object> result=new Map<String,Object>();
  try { Blob signedValue=Crypto.signWithCertificate('RSA-SHA256',Blob.valueOf('owned payload'),'GroveMissingKey');result.put('signed',signedValue!=null); }
  catch(Exception ex){result.put('type',ex.getTypeName());result.put('missingData',ex.getMessage().startsWith('Data Not Available:'));}
  System.assert(false,'OBSERVED|missingCertificate|'+JSON.serialize(result));
 }
}
