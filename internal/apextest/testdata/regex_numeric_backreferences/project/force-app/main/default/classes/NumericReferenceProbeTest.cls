@IsTest private class NumericReferenceProbeTest {
    @IsTest static void ref1_minimal() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)\\1','aa'))); }
    @IsTest static void ref2_minimal() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)\\2','abb'))); }
    @IsTest static void ref3_minimal() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)\\3','abcc'))); }
    @IsTest static void ref4_minimal() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)\\4','abcdd'))); }
    @IsTest static void ref5_minimal() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)\\5','abcdee'))); }
    @IsTest static void ref6_minimal() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)(f)\\6','abcdeff'))); }
    @IsTest static void ref7_minimal() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)(f)(g)\\7','abcdefgg'))); }
    @IsTest static void ref8_minimal() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)(f)(g)(h)\\8','abcdefghh'))); }
    @IsTest static void ref9_minimal() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)(f)(g)(h)(i)\\9','abcdefghii'))); }
    @IsTest static void ref1_ten() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)\\1','abcdefghija'))); }
    @IsTest static void ref2_ten() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)\\2','abcdefghijb'))); }
    @IsTest static void ref3_ten() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)\\3','abcdefghijc'))); }
    @IsTest static void ref4_ten() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)\\4','abcdefghijd'))); }
    @IsTest static void ref5_ten() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)\\5','abcdefghije'))); }
    @IsTest static void ref6_ten() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)\\6','abcdefghijf'))); }
    @IsTest static void ref7_ten() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)\\7','abcdefghijg'))); }
    @IsTest static void ref8_ten() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)\\8','abcdefghijh'))); }
    @IsTest static void ref9_ten() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)\\9','abcdefghiji'))); }
    @IsTest static void ref10_ten() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)\\10','abcdefghijj'))); }
    @IsTest static void ref10_ten_backspace() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)\\10','abcdefghij'+String.fromCharArray(new List<Integer>{8})+''))); }
    @IsTest static void ref10_nine_backspace() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)(f)(g)(h)(i)\\10','abcdefghi'+String.fromCharArray(new List<Integer>{8})+''))); }
    @IsTest static void ref10_nine_ref1_zero() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('(a)(b)(c)(d)(e)(f)(g)(h)(i)\\10','abcdefghia0'))); }
    @IsTest static void ref10_zero_backspace() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('\\10',''+String.fromCharArray(new List<Integer>{8})+''))); }
    @IsTest static void octal_zero_prefix() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('\\010',''+String.fromCharArray(new List<Integer>{8})+''))); }
    @IsTest static void escaped_backslash() { System.assertEquals('REFERENCE_ROWS',JSON.serialize(NumericReferenceProbe.observe('\\\\10','\\10'))); }
}
