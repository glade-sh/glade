@IsTest private class AccessorNamesTest62 {
 static Integer outerVisits=0, innerVisits=0, directSetterVisits=0, chainVisits=0;
 static String shared { get { outerVisits++; if (outerVisits==1) { String tempValue=new Child().shared; } return 'outer'; } set; }
 class Child { public String shared { get { innerVisits++; return AccessorNamesTest62.shared; } set; } }
 static String direct { get; set { directSetterVisits++; direct=value; } }
 static String left { get; set { chainVisits++; if (chainVisits==1) right=value; left=value; } }
 static String right { get; set { left=value; right=value; } }
 @IsTest static void nestedSameName() { System.assertEquals('outer',shared); System.assertEquals(2,outerVisits); System.assertEquals(1,innerVisits); }
 @IsTest static void directSetter() { direct='x'; System.assertEquals('x',direct); System.assertEquals(1,directSetterVisits); }
 @IsTest static void otherSetter() { left='y'; System.assertEquals('y',left); System.assertEquals('y',right); System.assertEquals(2,chainVisits); }
}
