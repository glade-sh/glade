@IsTest private class ScalarEdgesTest {
 @IsTest static void null_relations() {
  Id identity='001000000000001',emptyId=null; String text='Alpha',emptyText=null; Integer quantity=1,emptyNumber=null;
  System.assert(false,'OBSERVED|null_relations|'+JSON.serialize(new Map<String,Object>{'idGreater'=>identity>emptyId,'idLess'=>identity<emptyId,'idGreaterEqual'=>identity>=emptyId,'nullLessId'=>emptyId<identity,'stringGreater'=>text>emptyText,'nullLessString'=>emptyText<text,'numberGreater'=>quantity>emptyNumber,'bothNull'=>((String)null)>((String)null)}));
 }
 @IsTest static void text_relations() {
  String first='Zulu',second='alpha';
  String one=EncodingUtil.base64Encode(Blob.valueOf('a')),two=EncodingUtil.base64Encode(Blob.valueOf('b'));
  System.assert(false,'OBSERVED|text_relations|'+JSON.serialize(new Map<String,Object>{'less'=>first<second,'greater'=>first>second,'equalCaseLess'=>'a'<'A','equalCaseGreater'=>'a'>'A','encodedGreater'=>one>two,'encodedLess'=>one<two}));
 }
 @IsTest static void null_format() {
  Map<String,Object> observed=new Map<String,Object>(); List<Object> values=null;
  for(String pattern:new List<String>{'plain','{0}','{1}','before {0} after','prefix {0} suffix','{'}) {
   try {observed.put(pattern,String.format(pattern,values));}
   catch(Exception e){observed.put(pattern,e.getTypeName()+':'+e.getMessage());}
  }
  System.assert(false,'OBSERVED|null_format|'+JSON.serialize(observed));
 }
 @IsTest static void numeric_assertions() {
  Map<String,Object> observed=new Map<String,Object>();
  try {Double value=Math.log(-1); System.assertEquals(value,value); observed.put('nan','equal');} catch(Exception e){observed.put('nan',e.getTypeName());}
  observed.put('listEquality',new List<Integer>{1}==new List<Long>{1L});
  try {Date value=Date.valueOf('2008-20-58 12:20:20');observed.put('date',String.valueOf(value));}catch(Exception e){observed.put('date',e.getTypeName());}
  System.assert(false,'OBSERVED|numeric_assertions|'+JSON.serialize(observed));
 }
 @IsTest static void date_controls() {
  Map<String,Object> observed=new Map<String,Object>();
  for(String text:new List<String>{'2024-02-30','2024-02-30 01:00:00','2026-00-00','2026-00-00 00:00:00','2008-20-58','2008-20-58T12:20:20Z','2026-05-04 23:59:580','0000-01-01','10000-01-01','not-a-date'}) {
   try{observed.put(text,String.valueOf(Date.valueOf(text)));}catch(Exception e){observed.put(text,e.getTypeName());}
  }
  System.assert(false,'OBSERVED|date_controls|'+JSON.serialize(observed));
 }
 @IsTest static void format_controls() {
  Map<String,Object> observed=new Map<String,Object>(); List<Object> values=null; List<String> stringsList=null;
  for(String pattern:new List<String>{'\'quoted\' {0}','{bad}','{0,number}','}','{0,number,#.0}','{0,choice,0#{1}|1#x}','\'{0}\''}) {
   try{observed.put(pattern,String.format(pattern,values));}catch(Exception e){observed.put(pattern,e.getTypeName()+':'+e.getMessage());}
  }
  try{observed.put('stringsList',String.format('prefix {0}',stringsList));}catch(Exception e){observed.put('stringsList',e.getTypeName());}
  try{observed.put('literalNull',String.format('prefix {0}',null));}catch(Exception e){observed.put('literalNull',e.getTypeName());}
  System.assert(false,'OBSERVED|format_controls|'+JSON.serialize(observed));
 }
 @IsTest static void nan_controls() {
  Double first=Math.log(-1),second=Math.log(-1);
  Assert.areEqual(first,second);
  System.assert(false,'OBSERVED|nan_controls|'+JSON.serialize(new Map<String,Object>{'operatorEquals'=>first==second,'methodEquals'=>first.equals(second)}));
 }
 @IsTest static void extended_year() {
  Map<String,Object> observed=new Map<String,Object>(); Date value=Date.valueOf('10000-01-01');
  try{observed.put('year',value.year());}catch(Exception e){observed.put('year',e.getTypeName());}
  try{observed.put('same',String.valueOf(value.addDays(0)));}catch(Exception e){observed.put('same',e.getTypeName());}
  try{observed.put('next',String.valueOf(value.addDays(1)));}catch(Exception e){observed.put('next',e.getTypeName());}
  try{observed.put('difference',Date.newInstance(9999,12,31).daysBetween(value));}catch(Exception e){observed.put('difference',e.getTypeName());}
  System.assert(false,'OBSERVED|extended_year|'+JSON.serialize(observed));
 }
}
