package vm

import "testing"

// SF159 establishes the two collection identities below at API 63. The Id
// loop retains the construction used by ApexKit's IdFactory; the String set
// retains the Id-shaped String contrast from PolyfillsTests.
func TestApexKitIdentitySetsKeepIdAndStringContracts(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
String serverId = UserInfo.getUserId().left(7).right(4);
Set<Id> fakeIds = new Set<Id>();
for (Integer sequence = 2; sequence <= 400; sequence += 2) {
  String raw = '001' + serverId + '0'.repeat(18 - 7 - String.valueOf(sequence).length()) + String.valueOf(sequence);
  fakeIds.add((Id)raw);
}
System.assertEquals(200, fakeIds.size(), 'ApexKit factory-style fake Ids remain distinct');

// Local collection-key coherence: values that the Id set keeps distinct must
// not collide when used as Map keys.
Map<Id, Integer> fakeIdPositions = new Map<Id, Integer>();
for (Integer sequence = 2; sequence <= 400; sequence += 2) {
  String raw = '001' + serverId + '0'.repeat(18 - 7 - String.valueOf(sequence).length()) + String.valueOf(sequence);
  fakeIdPositions.put((Id)raw, sequence);
}
System.assertEquals(200, fakeIdPositions.size(), 'Id Map keys agree with Id Set identity');

Set<String> values = new Set<String>{
  '0011h00000xR1GfAAK',
  '0011h00000xR1GfAAL',
  '0011h00000xR1GfAAM',
  '0011h00000xR1GfAAN',
  '0011h00000xR1GfAAO'
};
System.assertEquals(true, values.contains('0011h00000xR1GfAAK'));
System.assertEquals(false, values.contains('0011h00000xR1GfAAZ'), 'Id-shaped String text remains String text');

Id canonical15 = (Id)'001000000000001';
Id canonical18 = (Id)'001000000000001AAA';
System.assertEquals(true, canonical15 == canonical18, 'valid canonical Id forms remain equal');
`, CompileOptions{APIVersion: "63.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
