for (String n : new List<String>{'CedarNPNsAttrObj','CedarNPNsAttrObjList','CedarNPNsAttrObjQualified','CedarNPNsAttrIface','CedarNPNsAttrEnum','CedarNPNsAttrCls','CedarNPNsRelRepeat','CedarNPNsRelField','CedarNPNsRelQualified','CedarNPNsStdOutputField','CedarNPNsStdOutputRel','CedarNPNsStdExpr'}) {
 String body;
 try { body = new PageReference('/apex/' + n).getContent().toString(); } catch (Exception e) { System.debug('PICKROW|' + n + '|EXC|' + EncodingUtil.base64Encode(Blob.valueOf(e.getTypeName() + ': ' + e.getMessage()))); continue; }
 System.debug('PICKROW|' + n + '|RAW|' + EncodingUtil.base64Encode(Blob.valueOf(body)));
}
