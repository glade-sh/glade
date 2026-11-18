Map<String,Object> out=FltProbe.read('Flt-CASE_ID-anonymous');out.put('published',JSON.deserializeUntyped('PUBLISHED_JSON'));System.debug('EVT_OBS|CASE_ID|anonymous|'+JSON.serialize(out));
