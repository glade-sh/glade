@IsTest private class PgCur64C2 {
 @IsTest static void probe(){
  String r;
  try {
   Object c = Database.getPaginationCursor('SELECT Id FROM Account', AccessLevel.USER_MODE);
   r = 'ok ' + (c == null ? 'null' : 'nonnull') + ' ' + String.valueOf(c).left(80);
  } catch (Exception e) { r = 'EXC ' + e.getTypeName() + ': ' + e.getMessage(); }
  System.assert(false, 'CAPTURE|' + r);
 }
}
