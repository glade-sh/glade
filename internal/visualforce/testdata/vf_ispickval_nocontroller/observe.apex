for (String n : new List<String>{'PickNC_guest','PickNC_standard','PickNC_lower_guest','PickNC_locale','PickNC_timezone','PickNC_language','PickNC_in_if','PickNC_in_not','PickNC_in_and','PickNC_rendered_true','PickNC_rendered_guest','PickNC_comp_guest','PickNC_comp_standard','PickNC_comp_locale','PickNC_comp_in_if','PickNC_comp_rendered'}) {
 String body;
 try { body = new PageReference('/apex/' + n).getContent().toString(); } catch (Exception e) { System.debug('PICKROW|' + n + '|EXC|' + EncodingUtil.base64Encode(Blob.valueOf(e.getTypeName() + ': ' + e.getMessage()))); continue; }
 System.debug('PICKROW|' + n + '|RAW|' + EncodingUtil.base64Encode(Blob.valueOf(body)));
}
