@IsTest private class CollectionEdgesTest {
 private class NumberIterator implements Iterator<Object> {
  public Boolean hasNext(){return true;}
  public Object next(){return 7;}
 }
 private class TextIterator implements Iterator<String> {
  public Boolean hasNext(){return true;}
  public String next(){return 'text';}
 }
 @IsTest static void iterator_store() {
  Map<String,Object> observed=new Map<String,Object>();
  try {
   List<Iterator<Object>> values=(List<Iterator<Object>>)Type.forName('List<Iterator<Integer>>').newInstance();
   values.add(new NumberIterator());observed.put('stored',values.size()); List<Object> erased=(List<Object>)values; erased.add(new TextIterator());observed.put('textStored',erased.size()); try{erased.add(new Contact());observed.put('ordinary','accepted');}catch(Exception e){observed.put('ordinary',e.getTypeName());}
  }catch(Exception e){observed.put('exception',e.getTypeName());}
  System.assert(false,'OBSERVED|iterator_store|'+JSON.serialize(observed));
 }
 @IsTest static void absent_children() {
  Contact value=(Contact)JSON.deserialize('{"attributes":{"type":"Contact"},"LastName":"Sample","Events":{"totalSize":1,"done":true,"records":[{"attributes":{"type":"Event"},"Subject":"Sample"}]}}',Contact.class);
  Map<String,Object> fields=value.getPopulatedFieldsAsMap().clone(); fields.keySet().remove('Events');
  fields.put('Id','003000000000007');
  Contact withoutChildren=(Contact)JSON.deserialize(JSON.serialize(fields),Contact.class);
  Contact neverPopulated=new Contact(LastName='Other');
  System.assert(false,'OBSERVED|absent_children|'+JSON.serialize(new Map<String,Object>{'removedNull'=>withoutChildren.getSObjects('Events')==null,'newNull'=>neverPopulated.getSObjects('Events')==null,'newIdNull'=>new Contact(Id='003000000000007').getSObjects('Events')==null,'originalCount'=>value.getSObjects('Events').size()}));
 }
 @IsTest static void widened_numeric() {
  List<Integer> integers=new List<Integer>{7}; List<Long> widened=integers;
  Object erased=integers; List<Long> casted=(List<Long>)erased;
  Object first=((List<Object>)widened)[0]; Object second=((List<Object>)casted)[0];
  Assert.areEqual(integers,widened); Assert.areEqual(new List<Integer>{7},widened);
  System.assert(false,'OBSERVED|widened_numeric|'+JSON.serialize(new Map<String,Object>{'assignedInteger'=>first instanceof Integer,'assignedLong'=>first instanceof Long,'castInteger'=>second instanceof Integer,'castLong'=>second instanceof Long,'identical'=>integers===widened}));
 }
}
