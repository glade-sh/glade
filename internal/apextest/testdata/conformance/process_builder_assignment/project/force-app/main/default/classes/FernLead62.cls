public class FernLead62 {
 public static String run() {
  List<String> out=new List<String>();
  for (String kind : new List<String>{'process','flow','control','matched'}) {
   for (Boolean header : new List<Boolean>{false,true}) {
    Lead l=new Lead(LastName=kind, Company=(kind=='matched'?'Route':'Other')); Database.DMLOptions opts=new Database.DMLOptions(); opts.assignmentRuleHeader.useDefaultRule=header;
    Database.insert(l,opts); Lead got=[SELECT Company,OwnerId FROM Lead WHERE Id=:l.Id];
    out.add(kind+':'+header+':'+got.Company+':'+(String.valueOf(got.OwnerId).startsWith('00G')?'queue':'user'));
   }
  }
  return String.join(out,'|');
 }
}