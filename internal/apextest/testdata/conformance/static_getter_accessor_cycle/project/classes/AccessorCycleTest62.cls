@IsTest private class AccessorCycleTest62 {
 static Integer alphaVisits=0, betaVisits=0, directVisits=0;
 static String alpha { get { alphaVisits++; if (alphaVisits==1) { String tempValue=beta; } return 'alpha'; } set; }
 static String beta { get { betaVisits++; String tempValue=alpha; return 'beta'; } set; }
 static String direct { get { directVisits++; if (directVisits==1) { String tempValue=direct; } return 'direct'; } set; }
 static String recursiveAlpha { get { return recursiveBeta; } set; }
 static String recursiveBeta { get { return recursiveAlpha; } set; }
 @IsTest static void finiteCycle() { System.assertEquals('alpha',alpha); System.assertEquals(2,alphaVisits); System.assertEquals(1,betaVisits); }
 @IsTest static void directAccess() { System.assertEquals('direct',direct); System.assertEquals(1,directVisits); }
 @IsTest static void recursiveCycle() { String tempValue=recursiveAlpha; }
}
