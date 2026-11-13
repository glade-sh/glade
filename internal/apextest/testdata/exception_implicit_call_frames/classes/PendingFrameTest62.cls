@IsTest public class PendingFrameTest62 {
    @IsTest static void case_literal() { System.assertEquals('native observation', PendingProbe62.capture('literal')); }
    @IsTest static void case_getter() { System.assertEquals('native observation', PendingProbe62.capture('getter')); }
    @IsTest static void case_setter() { System.assertEquals('native observation', PendingProbe62.capture('setter')); }
    @IsTest static void case_receiver() { System.assertEquals('native observation', PendingProbe62.capture('receiver')); }
    @IsTest static void case_initializer() { System.assertEquals('native observation', PendingProbe62.capture('initializer')); }
    @IsTest static void case_constructor() { System.assertEquals('native observation', PendingProbe62.capture('constructor')); }
    @IsTest static void case_broken() { System.assertEquals('native observation', PendingProbe62.capture('broken')); }
}
