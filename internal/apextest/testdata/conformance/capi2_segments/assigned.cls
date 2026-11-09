@IsTest private class CapiCapture { @IsTest static void probe() {
 ConnectApi.TextSegment s=new ConnectApi.TextSegment(); s.type=ConnectApi.MessageSegmentType.Mention; s.text='hi';
 System.assert(false,'CAPTURE|'+JSON.serialize(s)+'|'+JSON.serialize(s,true)+'|'+JSON.serialize(new List<ConnectApi.MessageSegment>{s})+'|field='+s.type);
} }