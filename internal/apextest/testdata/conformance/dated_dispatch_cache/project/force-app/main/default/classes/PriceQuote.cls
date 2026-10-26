public virtual class PriceQuote {
 public String code;
 public Decimal factor;
 public Date firstDay;
 public Date nextDay;
 public PriceQuote(String code, Decimal factor, Date firstDay, Date nextDay) {
  this.code=code;this.factor=factor;this.firstDay=firstDay;this.nextDay=nextDay;
 }
 protected virtual PriceQuote selectFor(SObject record) {return this;}
 protected virtual void addPeriod(PriceQuote value) {}
 private static Map<String,List<String>> paths {
  get {paths=paths??new Map<String,List<String>>{'OpportunityLineItem'=>new List<String>{'ServiceDate'}};return paths;} set;
 }
 private static Map<String,PriceQuote> quotes {
  get {quotes=quotes??new Map<String,PriceQuote>();return quotes;} set;
 }
 public static void configure(String typeName,List<String> path) {paths.put(typeName,path);}
 private static Datetime effectiveDate(SObject record) {
  List<String> path=new List<String>(paths.get(record.getSObjectType().toString()));
  while(path.size()>1) record=record.getSObject(path.remove(0));
  return record.isSet(path[0]) ? (Datetime)record.get(path[0]) : null;
 }
 public static void install(PriceQuote initial,List<PriceQuote> periods) {
  quotes.put(initial.code,initial);
  for(PriceQuote period:periods) {
   PriceQuote current=quotes.get(period.code);
   if(current!=null && !(current instanceof CalendarQuote)) quotes.put(period.code,new CalendarQuote(current));
  }
  for(PriceQuote period:periods) {
   PriceQuote current=quotes.get(period.code);
   if(current==null) quotes.put(period.code,new CalendarQuote(period));
   else current.addPeriod(period);
  }
 }
 public static Decimal convert(SObject record,String code,Decimal amount) {
  Schema.SObjectField field=record.getSObjectType().getDescribe().fields.getMap().get('TotalPrice');
  if(field.getDescribe().getType()!=Schema.DisplayType.CURRENCY) return amount;
  return quotes.get(code).selectFor(record).factor*amount;}
 private class CalendarQuote extends PriceQuote {
  private List<PriceQuote> periods=new List<PriceQuote>();
  private PriceQuote fallback;
  public CalendarQuote(PriceQuote value) {super(value.code,value.factor,null,null);this.addPeriod(value);}
  protected override void addPeriod(PriceQuote value) {if(value.firstDay==null) this.fallback=value;else this.periods.add(value);}
  protected override PriceQuote selectFor(SObject record) {
   Datetime day=effectiveDate(record);
   for(PriceQuote value:this.periods) if(day!=null && day>=value.firstDay && day<value.nextDay) return value;
   return this.fallback;
  }
 }
}
