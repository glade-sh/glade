@IsTest private class CapiCapture { @IsTest static void probe() {
 List<String> rows = new List<String>();
 for(String name : new List<String>{'ConnectApi.ComplexSegment','ConnectApi.EntityLinkSegment','ConnectApi.FieldChangeNameSegment','ConnectApi.FieldChangeValueSegment','ConnectApi.HashtagSegment','ConnectApi.InlineImageSegment','ConnectApi.LinkSegment','ConnectApi.MarkupBeginSegment','ConnectApi.MarkupEndSegment','ConnectApi.MentionSegment','ConnectApi.MoreChangesSegment','ConnectApi.ResourceLinkSegment','ConnectApi.TextSegment'}) {
  try { Object o = Type.forName(name).newInstance(); Map<String,Object> m = (Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(o)); rows.add(name + '=' + String.valueOf(m.get('type'))); }
  catch(Exception e) { rows.add(name + '=EX:' + e.getTypeName()); }
 }
 System.assert(false, 'CAPTURE|' + String.join(rows,';'));
} }