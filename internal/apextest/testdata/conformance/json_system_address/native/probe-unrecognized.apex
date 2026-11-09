List<String> cases = new List<String>{
  '{"bogus":"x"}',
  '{"bogus":"xyz"}',
  '{"bogus": 12345}',
  '{"city":"A","bogus":true,"z":1}',
  '{"city":"A",\n "bogus":"x"}',
  '{"bogus":null}',
  '{"bogus":{"a":1}}',
  '[1]',
  '"s"'
};
for (String c : cases) {
  try { JSON.deserialize(c, Address.class); System.debug('OBS|' + c.replace('\n','~') + '|OK'); }
  catch (Exception e) { System.debug('OBS|' + c.replace('\n','~') + '|' + e.getTypeName() + '|' + e.getMessage()); }
}
