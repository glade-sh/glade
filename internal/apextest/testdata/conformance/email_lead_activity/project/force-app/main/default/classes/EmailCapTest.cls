@IsTest
private class EmailCapTest {
    @IsTest static void a() { Test.startTest(); String o = EmailCap.run('A'); Test.stopTest(); System.assert(false, o); }
    @IsTest static void b() { Test.startTest(); String o = EmailCap.run('B'); Test.stopTest(); System.assert(false, o); }
    @IsTest static void c() { Test.startTest(); String o = EmailCap.run('C'); Test.stopTest(); System.assert(false, o); }
    @IsTest static void d() { Test.startTest(); String o = EmailCap.run('D'); Test.stopTest(); System.assert(false, o); }
    @IsTest static void e() { Test.startTest(); String o = EmailCap.run('E'); Test.stopTest(); System.assert(false, o); }
}
