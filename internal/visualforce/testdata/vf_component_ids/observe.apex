for (String n : new List<String>{'CedarPItemsSameIdPg','CedarPItemsSameIdCmp','CedarPItemsOwnIdsPg','CedarPItemsOwnIdsCmp','CedarPBodyAndSiblingPg','CedarPBodyAndSiblingCmp','CedarPBodyTwicePg','CedarPBodyTwiceCmp','CedarPBodyExclusivePg','CedarPBodyExclusiveCmp','CedarPActionAccountNewPg','CedarPActionAccountListPg','CedarPActionCustomNewPg','CedarPActionCustomNewBarePg','CedarPActionCustomViewPg','CedarPScriptRepeatFieldPg','CedarPScriptRepeatTextPg','CedarPActionTabNewPg','CedarPActionTabNewBarePg','CedarPActionTabViewPg','CedarPActionTabTabPg','CedarPActionAccountViewPg','CedarPActionAccountTabPg','CedarPActionAcctEditPg','CedarPActionAcctDeletePg'}) {
 String body;
 try { body = new PageReference('/apex/' + n).getContent().toString(); } catch (Exception e) { System.debug('PICKROW|' + n + '|EXC|' + EncodingUtil.base64Encode(Blob.valueOf(e.getTypeName() + ': ' + e.getMessage()))); continue; }
 System.debug('PICKROW|' + n + '|RAW|' + EncodingUtil.base64Encode(Blob.valueOf(body)));
}
