@IsTest private class TraceTest67 {
    @IsTest static void case_null() { System.assertEquals('EXPECTED',TraceProbe67.capture('null')); }
    @IsTest static void case_index() { System.assertEquals('EXPECTED',TraceProbe67.capture('index')); }
    @IsTest static void case_divide() { System.assertEquals('EXPECTED',TraceProbe67.capture('divide')); }
    @IsTest static void case_cast() { System.assertEquals('EXPECTED',TraceProbe67.capture('cast')); }
    @IsTest static void case_query() { System.assertEquals('EXPECTED',TraceProbe67.capture('query')); }
    @IsTest static void case_required() { System.assertEquals('EXPECTED',TraceProbe67.capture('required')); }
    @IsTest static void case_trigger() { System.assertEquals('EXPECTED',TraceProbe67.capture('trigger')); }
    @IsTest static void case_chain() { System.assertEquals('EXPECTED',TraceProbe67.capture('chain')); }
}
