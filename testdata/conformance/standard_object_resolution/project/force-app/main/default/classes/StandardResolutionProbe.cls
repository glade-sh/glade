public class StandardResolutionProbe {
 public static String observe(String route) {
  String payload='{"attributes":{"type":"Asset"},"Name":"probe","Quantity":1.5,"PurchaseDate":"2026-01-02"}';
  if(route=='strictUnknown') {
   try {JSON.deserializeStrict('{"attributes":{"type":"Asset"},"UnknownColumn":true}',SObject.class);return 'accepted';}
   catch(Exception issue) {return JSON.serialize(new Map<String,Object>{'error'=>issue.getTypeName(),'message'=>issue.getMessage()});}
  }
  SObject row;
  if(route=='deserialize') row=(SObject)JSON.deserialize(payload,SObject.class);
  else if(route=='deserializeStrict') row=(SObject)JSON.deserializeStrict(payload,SObject.class);
  else if(route=='deserializeList') row=((List<SObject>)JSON.deserialize('['+payload+']',List<SObject>.class))[0];
  else if(route=='deserializeStrictList') row=((List<SObject>)JSON.deserializeStrict('['+payload+']',List<SObject>.class))[0];
  else if(route=='deserializeMap') row=((Map<String,SObject>)JSON.deserialize('{"item":'+payload+'}',Map<String,SObject>.class)).get('item');
  else if(route=='parser') {JSONParser parser=JSON.createParser(payload); row=(SObject)parser.readValueAs(SObject.class);}
  else if(route=='typedDeserialize') row=(Asset)JSON.deserialize(payload,Asset.class);
  else if(route=='newSObject') {row=Asset.SObjectType.newSObject();row.put('Name','probe');row.put('Quantity',1.5);row.put('PurchaseDate',Date.newInstance(2026,1,2));}
  else if(route=='newSObjectDefaults') {row=Asset.SObjectType.newSObject(null,true);row.put('Name','probe');row.put('Quantity',1.5);row.put('PurchaseDate',Date.newInstance(2026,1,2));}
  else if(route=='clone') row=new Asset(Name='probe',Quantity=1.5,PurchaseDate=Date.newInstance(2026,1,2)).clone(false,true,false,false);
  else if(route=='listDeepClone') row=new List<Asset>{new Asset(Name='probe',Quantity=1.5,PurchaseDate=Date.newInstance(2026,1,2))}.deepClone(false,true,false)[0];
  else {System.assert(false,'Unknown route '+route);return null;}
  String absent;
  try {absent=String.valueOf(row.get('SerialNumber'));} catch(Exception issue) {absent=issue.getTypeName();}
  Map<String,Object> result=new Map<String,Object>{'type'=>String.valueOf(row.getSObjectType()),'name'=>row.get('Name'),'absent'=>absent,'idNull'=>row.Id==null,'quantityDecimal'=>row.get('Quantity') instanceof Decimal,'purchaseDate'=>row.get('PurchaseDate') instanceof Date};
  return JSON.serialize(result);
 }
}
