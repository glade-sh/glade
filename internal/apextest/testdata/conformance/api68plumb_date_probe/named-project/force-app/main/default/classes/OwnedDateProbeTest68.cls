@IsTest private class OwnedDateProbeTest68 {
 @IsTest static void named() { System.assertEquals('System.JSONException|Invalid format: "null"', OwnedDateProbe68.rejected()); System.assertEquals('2026-11-03', OwnedDateProbe68.accepted()); }
 @IsTest static void directTest() { String result='returned'; try { JSON.deserialize('null', Date.class); } catch(Exception e) { result=e.getTypeName()+'|'+e.getMessage(); } System.assertEquals('System.JSONException|Invalid format: "null"',result); System.assertEquals('2026-11-03',String.valueOf((Date)JSON.deserialize('"2026-11-03"',Date.class))); }
}
