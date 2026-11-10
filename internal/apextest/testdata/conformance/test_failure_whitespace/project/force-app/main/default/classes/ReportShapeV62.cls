@IsTest private class ReportShapeV62 {
 class ProbeException extends Exception {}
 @IsTest static void dmlOne() { throw new DmlException('first  '); }
 @IsTest static void dmlTwo() { throw new DmlException('  second  '); }
 @IsTest static void dmlCaught() { try { throw new DmlException('first  '); } catch (DmlException e) { System.assert(false,'CAPTURE|'+JSON.serialize(e.getMessage())); } }
 @IsTest static void customException() { throw new ProbeException('  custom  '); }
 @IsTest static void customCaught() { try { throw new ProbeException('  custom  '); } catch (Exception e) { System.assert(false,'CAPTURE|'+JSON.serialize(e.getMessage())); } }
 @IsTest static void assertion() { System.assert(false,'  assertion  '); }
}