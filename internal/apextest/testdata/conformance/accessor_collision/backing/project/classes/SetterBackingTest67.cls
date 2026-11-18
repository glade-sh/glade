@IsTest private class SetterBackingTest67 {
 static Integer ownReads=0; static String seenOwn;
 static String own { get { ownReads++; return 'own-getter'; } set { seenOwn=own; own=value; } }
 @IsTest static void backingRead() { own='x'; System.assertEquals(null,seenOwn); System.assertEquals(0,ownReads); }
}
