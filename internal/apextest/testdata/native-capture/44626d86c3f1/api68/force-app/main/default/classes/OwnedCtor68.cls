public class OwnedCtor68 {
 public class Parcel {
  public List<String> marks=new List<String>();
  public List<SObject> records=new List<SObject>();
  public Parcel(List<SObject> records){marks.add('list');this.records.addAll(records);}
  public Parcel(Integer count){this(new List<SObject>{new Account(Name='owned')});marks.add('chain');}
 }
 public static Map<String,Object> observe(String shape){
  List<SObject> records=new List<SObject>{new Account(Name='first'),new Account(Name='second')};
  Parcel value;
  if(shape=='CHAIN')value=new Parcel(1);else value=new Parcel(records);
  Map<String,Object> out=new Map<String,Object>{'marks'=>value.marks,'size'=>value.records.size()};
  if(shape=='WRAPPER'){out.put('inputSize',records.size());out.put('firstSame',value.records[0]===records[0]);out.put('secondSame',value.records[1]===records[1]);}
  return out;
 }
}