@IsTest private class AccessorCollisionTest {
 static Integer ownReads=0, otherReads=0;
 static String seenOwn, seenOther;
 static String own { get { ownReads++; return 'own-getter'; } set { seenOwn=own; own=value; } }
 static String other { get { otherReads++; return 'other-getter'; } set; }
 static String target { get; set { seenOther=other; target=value; } }
 @IsTest static void collision() { System.assertEquals('top',Holder.topLevel.shared); System.assertEquals(2,Holder.topVisits); System.assertEquals(1,Holder.nestedVisits); }
 @IsTest static void ownSetterRead() { own='x'; System.assertEquals('own-getter',seenOwn); System.assertEquals(1,ownReads); }
 @IsTest static void otherSetterRead() { target='x'; System.assertEquals('other-getter',seenOther); System.assertEquals(1,otherReads); }
}
