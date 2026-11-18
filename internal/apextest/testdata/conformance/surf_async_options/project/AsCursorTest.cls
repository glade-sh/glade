@IsTest
private class AsCursorTest {
 @IsTest static void observe() {
  List<String> rows = new List<String>();
  List<Account> accts = new List<Account>();
  for (Integer i = 0; i < 5; i++) { accts.add(new Account(Name = 'C' + i)); }
  insert accts;
  Test.startTest();
  Database.Cursor c = Database.getCursor('SELECT Id, Name FROM Account ORDER BY Name');
  rows.add('num=' + c.getNumRecords());
  List<Account> page = (List<Account>) c.fetch(1, 2);
  rows.add('page=' + page.size() + ':' + page[0].Name + ',' + page[1].Name);
  rows.add('num2=' + c.getNumRecords());
  try {
   List<Account> tail = (List<Account>) c.fetch(4, 2);
   rows.add('tail=' + tail.size() + ':' + tail[0].Name);
  } catch (Exception e) { rows.add('tail=' + e.getTypeName() + '|' + e.getMessage()); }
  Test.stopTest();
  System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
