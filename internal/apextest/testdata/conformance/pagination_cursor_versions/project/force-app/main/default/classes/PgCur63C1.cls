@IsTest private class PgCur63C1 {
 @IsTest static void probe(){
  String r;
  try {
   Object c = Database.getPaginationCursor('SELECT Id FROM Account');
   r = 'ok ' + (c == null ? 'null' : 'nonnull') + ' ' + String.valueOf(c).left(80);
  } catch (Exception e) { r = 'EXC ' + e.getTypeName() + ': ' + e.getMessage(); }
  System.assert(false, 'CAPTURE|' + r);
 }
}
