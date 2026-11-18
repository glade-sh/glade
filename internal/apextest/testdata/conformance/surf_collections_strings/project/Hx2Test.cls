@IsTest
private class Hx2Test {
 @IsTest static void observe() {
List<String> rows = new List<String>();
Long M = 4294967296L;
Id i15 = '001000000000001';
Id i18 = '001000000000001AAA';
rows.add('i15=' + String.valueOf(i15) + '|len=' + String.valueOf(i15).length() + '|hash=' + i15.hashCode() + ';i18=' + String.valueOf(i18) + '|hash=' + i18.hashCode() + '|eq=' + (i15 == i18));
rows.add('str18=' + '001000000000001AAA'.hashCode() + '|str15=' + '001000000000001'.hashCode());
Account a15 = new Account(Id='001000000000001');
Account a18 = new Account(Id='001000000000001AAA');
Account aI15 = new Account(Id=i15);
rows.add('a15=' + a15.hashCode() + '|a18=' + a18.hashCode() + '|aI15=' + aI15.hashCode() + '|eq=' + (a15 == a18) + '|eqI=' + (aI15 == a18) + '|idStr=' + String.valueOf(a15.Id));
Set<Account> s = new Set<Account>{a18};
rows.add('setContains15=' + s.contains(a15) + '|mapGet=' + (new Map<Account,String>{a18 => 'v'}).get(a15));
Contact c15 = new Contact(LastName='A', AccountId='001000000000001');
Contact c18 = new Contact(LastName='A', AccountId='001000000000001AAA');
rows.add('c15=' + c15.hashCode() + '|c18=' + c18.hashCode() + '|eq=' + (c15 == c18));
rows.add('nameStr15=' + new Account(Name='abcdefghijklmno').hashCode() + '|nameStr18=' + new Account(Name='001000000000001AAA').hashCode() + '|nameStr15id=' + new Account(Name='001000000000001').hashCode());
Id other = '001000000000002AAA';
rows.add('mapId=' + (new Map<Id,String>{i15 => 'x'}).containsKey(i18) + '|setId=' + (new Set<Id>{i15}).contains(i18) + '|listHash=' + (new List<Id>{i15}).hashCode() + '|' + (new List<Id>{i18}).hashCode() + '|otherHash=' + other.hashCode());
 System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
