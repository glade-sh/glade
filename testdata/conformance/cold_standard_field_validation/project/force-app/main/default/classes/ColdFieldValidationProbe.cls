public class ColdFieldValidationProbe {
 public static String observe(String route) {
  String payload='{"attributes":{"type":"Asset"},"Name":"probe","Quantity":1.5,"PurchaseDate":"2026-01-02"}';
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
  else if(route=='deserializeStrictMap') row=((Map<String,SObject>)JSON.deserializeStrict('{"item":'+payload+'}',Map<String,SObject>.class)).get('item');
  else if(route=='parserStrict') {JSONParser parser=JSON.createParser(payload);row=(SObject)parser.readValueAsStrict(SObject.class);}
  else if(route=='constructor') row=new Asset(Name='probe',Quantity=1.5);
  else if(route=='jsonClone') row=((SObject)JSON.deserialize(payload,SObject.class)).clone(false,true,false,false);
  else if(route=='jsonListDeepClone') row=((List<SObject>)JSON.deserialize('['+payload+']',List<SObject>.class)).deepClone(false,true,false)[0];
  else {System.assert(false,'Unknown route '+route);return null;}
  Map<String,String> result=new Map<String,String>();
  try {row.get('DefinitelyMissing');result.put('unknownGet','accepted');} catch(Exception issue) {result.put('unknownGet',issue.getTypeName()+': '+issue.getMessage());}
  try {row.put('DefinitelyMissing','x');result.put('unknownPut','accepted');} catch(Exception issue) {result.put('unknownPut',issue.getTypeName()+': '+issue.getMessage());}
  try {row.put('Quantity','not-a-number');result.put('typedPut','accepted');} catch(Exception issue) {result.put('typedPut',issue.getTypeName()+': '+issue.getMessage());}
  try {row.put('CreatedDate',Datetime.newInstance(2026,1,2,0,0,0));result.put('readonlyPut','accepted');} catch(Exception issue) {result.put('readonlyPut',issue.getTypeName()+': '+issue.getMessage());}
  try {row.put(Asset.Quantity,'not-a-number');result.put('typedTokenPut','accepted');} catch(Exception issue) {result.put('typedTokenPut',issue.getTypeName()+': '+issue.getMessage());}
  try {row.put(Asset.CreatedDate,Datetime.newInstance(2026,1,2,0,0,0));result.put('readonlyTokenPut','accepted');} catch(Exception issue) {result.put('readonlyTokenPut',issue.getTypeName()+': '+issue.getMessage());}
  Object previous=row.put('quantity',2.5);
  result.put('validPut','previous='+String.valueOf(previous)+';current='+String.valueOf(row.get(Asset.Quantity)));
  result.put('validGet',String.valueOf(row.get('Name')));
  return JSON.serialize(result);
 }
}
