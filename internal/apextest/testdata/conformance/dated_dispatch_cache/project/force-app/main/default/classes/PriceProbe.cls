public class PriceProbe {
 public static Map<String,Object> observe() {
  Date today=Date.today();
  PriceQuote initial=new PriceQuote('Q',1,null,null);
  PriceQuote early=new PriceQuote('Q',1.3,today.addDays(-20),today.addDays(-4));
  PriceQuote late=new PriceQuote('Q',1.6,today.addDays(-4),today.addDays(3));
  PriceQuote origin=new PriceQuote('O',1,Date.newInstance(0,12,30),Date.newInstance(9999,12,31));
  PriceQuote.install(initial,new List<PriceQuote>{origin,early,late});
  Opportunity parent=new Opportunity(Name='Range',StageName='Prospecting',CloseDate=today.addDays(-10));
  SObject line=(SObject)JSON.deserialize(JSON.serialize(new OpportunityLineItem(Opportunity=parent,ServiceDate=today,Quantity=1,TotalPrice=9)),SObject.class);
  Decimal defaultValue=PriceQuote.convert(line,'Q',9);
  List<OpportunityLineItem> typed=new List<OpportunityLineItem>{(OpportunityLineItem)line};
  List<SObject> wide=typed;
  PriceQuote.configure(typed.getSObjectType().toString(),new List<String>{'Opportunity','CloseDate'});
  String cacheKey=line.Id+((Object)line).hashCode().format();
  SObject copied=line.clone(true,true,true,true);
  copied.put('Description','converted');
  copied.put('TotalPrice',PriceQuote.convert(line,'Q',9));
  Map<String,SObject> converted=new Map<String,SObject>{cacheKey=>copied};
  Boolean originalStable=(cacheKey==line.Id+((Object)line).hashCode().format());
  SObject found=converted.get(line.Id+((Object)line).hashCode().format())??line;
  Decimal configuredValue=PriceQuote.convert(line,'Q',9);
  return new Map<String,Object>{'priceKinds'=>new Map<String,Object>{'UnitPrice'=>OpportunityLineItem.UnitPrice.getDescribe().getType().name(),'ListPrice'=>OpportunityLineItem.ListPrice.getDescribe().getType().name(),'Quantity'=>OpportunityLineItem.Quantity.getDescribe().getType().name()},'listType'=>typed.getSObjectType().toString(),'wideType'=>wide.getSObjectType().toString(),'fieldKind'=>OpportunityLineItem.TotalPrice.getDescribe().getType().name(),'fieldWritable'=>OpportunityLineItem.TotalPrice.getDescribe().isUpdateable(),'cachedValue'=>found.get('TotalPrice'),'originalStable'=>originalStable,'originalPrice'=>line.get('TotalPrice'),'defaultValue'=>defaultValue,'configuredValue'=>configuredValue,'originValue'=>PriceQuote.convert(line,'O',9),'repeatValue'=>PriceQuote.convert(line,'Q',9)};
 }
}
