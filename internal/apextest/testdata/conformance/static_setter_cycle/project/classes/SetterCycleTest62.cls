@IsTest private class SetterCycleTest62 {
 static Integer directVisits=0, finiteVisits=0;
 static String direct { get; set { directVisits++; direct=value; } }
 static String left { get; set { finiteVisits++; if (finiteVisits==1) right=value; left=value; } }
 static String right { get; set { left=value; right=value; } }
 static String recursiveLeft { get; set { recursiveRight=value; recursiveLeft=value; } }
 static String recursiveRight { get; set { recursiveLeft=value; recursiveRight=value; } }
 @IsTest static void directControl() { direct='x'; System.assertEquals(1,directVisits); System.assertEquals('x',direct); }
 @IsTest static void finiteCycle() { left='y'; System.assertEquals(2,finiteVisits); System.assertEquals('y',left); System.assertEquals('y',right); }
 @IsTest static void recursiveCycle() { recursiveLeft='z'; }
}
