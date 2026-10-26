public class ColdRelationshipValidationProbe {
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
  try {row.putSObject('DefinitelyMissing',new Account(Name='parent'));result.put('unknownPutSObject','accepted');} catch(Exception issue) {result.put('unknownPutSObject',issue.getTypeName()+': '+issue.getMessage());}
  try {row.putSObject('Account',new Contact(LastName='parent'));result.put('wrongTargetPutSObject','accepted');} catch(Exception issue) {result.put('wrongTargetPutSObject',issue.getTypeName()+': '+issue.getMessage());}
  try {row.putSObject(Asset.AccountId,new Contact(LastName='parent'));result.put('wrongTargetTokenPutSObject','accepted');} catch(Exception issue) {result.put('wrongTargetTokenPutSObject',issue.getTypeName()+': '+issue.getMessage());}
  try {row.isSet('DefinitelyMissing');result.put('unknownIsSet','accepted');} catch(Exception issue) {result.put('unknownIsSet',issue.getTypeName()+': '+issue.getMessage());}
  try {row.getSObject('DefinitelyMissing');result.put('unknownGetSObject','accepted');} catch(Exception issue) {result.put('unknownGetSObject',issue.getTypeName()+': '+issue.getMessage());}
  try {row.getSObjects('DefinitelyMissing');result.put('unknownGetSObjects','accepted');} catch(Exception issue) {result.put('unknownGetSObjects',issue.getTypeName()+': '+issue.getMessage());}
  try {result.put('validIsSet',String.valueOf(row.isSet('Name')));} catch(Exception issue) {result.put('validIsSet',issue.getTypeName()+': '+issue.getMessage());}
  try {result.put('validTokenIsSet',String.valueOf(row.isSet(Asset.Name)));} catch(Exception issue) {result.put('validTokenIsSet',issue.getTypeName()+': '+issue.getMessage());}
  try {result.put('validGetSObject',String.valueOf(row.getSObject('Account')));} catch(Exception issue) {result.put('validGetSObject',issue.getTypeName()+': '+issue.getMessage());}
  try {result.put('validTokenGetSObject',String.valueOf(row.getSObject(Asset.AccountId)));} catch(Exception issue) {result.put('validTokenGetSObject',issue.getTypeName()+': '+issue.getMessage());}
  try {result.put('validGetSObjects',String.valueOf(row.getSObjects('ChildAssets')));} catch(Exception issue) {result.put('validGetSObjects',issue.getTypeName()+': '+issue.getMessage());}
  try {result.put('validTokenGetSObjects',String.valueOf(row.getSObjects(Asset.ParentId)));} catch(Exception issue) {result.put('validTokenGetSObjects',issue.getTypeName()+': '+issue.getMessage());}
  try {row.putSObject('Account',new Account(Name='parent'));result.put('validPutSObject',String.valueOf(row.getSObject('Account').get('Name')));} catch(Exception issue) {result.put('validPutSObject',issue.getTypeName()+': '+issue.getMessage());}
  try {row.putSObject(Asset.AccountId,new Account(Name='parent'));result.put('validTokenPutSObject',String.valueOf(row.getSObject('Account').get('Name')));} catch(Exception issue) {result.put('validTokenPutSObject',issue.getTypeName()+': '+issue.getMessage());}
  return JSON.serialize(result);
 }
}
