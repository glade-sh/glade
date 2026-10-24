@IsTest private class ConservativeSplitProbeTest {
@IsTest static void fold_dotted_i(){System.assertEquals('SPLIT_ROWS',JSON.serialize(ConservativeSplitProbe.observe('(?iu)'+String.fromCharArray(new List<Integer>{304})+'','i')));}
@IsTest static void octal_class_400(){System.assertEquals('SPLIT_ROWS',JSON.serialize(ConservativeSplitProbe.observe('[\\400]',''+String.fromCharArray(new List<Integer>{256})+'')));}
@IsTest static void octal_class_777(){System.assertEquals('SPLIT_ROWS',JSON.serialize(ConservativeSplitProbe.observe('[\\777]',''+String.fromCharArray(new List<Integer>{511})+'')));}
@IsTest static void ascii(){System.assertEquals('SPLIT_ROWS',JSON.serialize(ConservativeSplitProbe.observe('-','-')));}
@IsTest static void ascii_fold(){System.assertEquals('SPLIT_ROWS',JSON.serialize(ConservativeSplitProbe.observe('(?i)a','A')));}
@IsTest static void unicode(){System.assertEquals('SPLIT_ROWS',JSON.serialize(ConservativeSplitProbe.observe(''+String.fromCharArray(new List<Integer>{233})+'',''+String.fromCharArray(new List<Integer>{233})+'')));}
@IsTest static void escaped_dot(){System.assertEquals('SPLIT_ROWS',JSON.serialize(ConservativeSplitProbe.observe('\\.','.')));}
@IsTest static void octal_zero(){System.assertEquals('SPLIT_ROWS',JSON.serialize(ConservativeSplitProbe.observe('\\010',''+String.fromCharArray(new List<Integer>{8})+'')));}
@IsTest static void escaped_backslash(){System.assertEquals('SPLIT_ROWS',JSON.serialize(ConservativeSplitProbe.observe('\\\\10','\\10')));}
@IsTest static void ascii_class(){System.assertEquals('SPLIT_ROWS',JSON.serialize(ConservativeSplitProbe.observe('[1]0','10')));}
@IsTest static void quoted(){System.assertEquals('SPLIT_ROWS',JSON.serialize(ConservativeSplitProbe.observe('\\Qabc\\E','abc')));}
@IsTest static void ref10(){System.assertEquals('SPLIT_ROWS',JSON.serialize(ConservativeSplitProbe.observe('(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)\\10','abcdefghijj')));}
@IsTest static void ref10_nonmatch(){System.assertEquals('SPLIT_ROWS',JSON.serialize(ConservativeSplitProbe.observe('(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)\\10','abcdefghij'+String.fromCharArray(new List<Integer>{8})+'')));}
}
