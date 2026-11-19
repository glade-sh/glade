@IsTest private class DefaultProbeTest62 {
 @IsTest static void observe() {
  System.assertEquals('object:null|object:null|object:true|null', DefaultProbe62.run());
  try { Integer.valueOf(new List<String>()); System.assert(false, 'expected TypeException'); } catch (System.TypeException e) { System.assert(e.getMessage().startsWith('Invalid integer:')); }
 }
}
