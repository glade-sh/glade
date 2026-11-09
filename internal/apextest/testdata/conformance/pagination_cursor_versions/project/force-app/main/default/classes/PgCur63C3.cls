@IsTest private class PgCur63C3 {
 @IsTest static void probe(){
  String r;
  try {
   Object c = Database.getPaginationCursorWithBinds('SELECT Id FROM Account WHERE Name = :n', new Map<String,Object>{'n'=>'x'}, AccessLevel.USER_MODE);
   r = 'ok ' + (c == null ? 'null' : 'nonnull') + ' ' + String.valueOf(c).left(80);
  } catch (Exception e) { r = 'EXC ' + e.getTypeName() + ': ' + e.getMessage(); }
  System.assert(false, 'CAPTURE|' + r);
 }
}
