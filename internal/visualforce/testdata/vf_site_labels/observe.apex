for (String n : new List<String>{'SiteLbl_000','SiteLbl_001','SiteLbl_002','SiteLbl_003','SiteLbl_004','SiteLbl_005','SiteLbl_006','SiteLbl_007','SiteLbl_008','SiteLbl_009','SiteLbl_010','SiteLbl_011'}) {
 String body;
 try { body = new PageReference('/apex/' + n).getContent().toString(); } catch (Exception e) { System.debug('PICKROW|' + n + '|EXC|' + EncodingUtil.base64Encode(Blob.valueOf(e.getTypeName() + ': ' + e.getMessage()))); continue; }
 System.debug('PICKROW|' + n + '|RAW|' + EncodingUtil.base64Encode(Blob.valueOf(body)));
}
