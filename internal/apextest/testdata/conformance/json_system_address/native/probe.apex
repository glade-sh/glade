List<String> cases = new List<String>{
  '{"city":"San Francisco","country":"USA","postalCode":"94103","state":"CA","street":"123 Market St"}',
  '{"city":"A","stateCode":"CA","countryCode":"US","latitude":37.5,"longitude":-122.25,"geocodeAccuracy":"Street"}',
  '{}',
  '{"city":"A","bogus":"x"}',
  '{"city":1}',
  'null'
};
for (String c : cases) {
  try {
    Address a = (Address) JSON.deserialize(c, Address.class);
    System.debug('OBS|' + c + '|OK|' + (a == null ? 'null' : a.city + ',' + a.country + ',' + a.countryCode + ',' + a.postalCode + ',' + a.state + ',' + a.stateCode + ',' + a.street + ',' + a.latitude + ',' + a.longitude + ',' + a.geocodeAccuracy));
  } catch (Exception e) { System.debug('OBS|' + c + '|EX|' + e.getTypeName() + '|' + e.getMessage()); }
}
try {
  Address b = (Address) JSON.deserialize('{"city":"X","street":"S"}', Address.class);
  System.debug('OBS|ser|' + JSON.serialize(b));
} catch (Exception e) { System.debug('OBS|ser|EX|' + e.getTypeName() + '|' + e.getMessage()); }
