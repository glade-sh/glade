@IsTest private class DepthTest62 { @IsTest static void boundary() {
String obs='test:'+Test.isRunningTest()+'|'+DepthTrace.probe(15)+'|'+DepthTrace.probe(16)+'|'+DepthTrace.probe(17);
System.assertEquals('EXPECTED',obs); } }
