@IsTest private class R6MultiTest {
 @IsTest static void none(){System.assertEquals('PASS',R6MultiProbe.observe('R6 None'));}
 @IsTest static void one(){insert new Account(Name='R6 One');System.assertEquals('PASS',R6MultiProbe.observe('R6 One'));}
 @IsTest static void two(){insert new List<Account>{new Account(Name='R6 Two'),new Account(Name='R6 Two')};System.assertEquals('MATCH',R6MultiProbe.observe('R6 Two'));}
}
