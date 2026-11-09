@IsTest private class CapiSubtypeProbe {
 @IsTest static void probe() {
  ConnectApi.TextSegment s = new ConnectApi.TextSegment(); s.text = 'hi';
  ConnectApi.LinkSegment l = new ConnectApi.LinkSegment();
  ConnectApi.FeedBody b = new ConnectApi.FeedBody();
  System.assert(false, 'CAPTURE|' + JSON.serialize(s) + '|' + JSON.serialize(l) + '|' + JSON.serialize(b) + '|field=' + String.valueOf(s.type));
 }
}