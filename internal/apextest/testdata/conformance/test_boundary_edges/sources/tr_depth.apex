Edge_Rec__A____c rec = new Edge_Rec__A____c(Name = 'r');
insert rec;
EdgeDepth__A__.n = 0;
String id15 = String.valueOf(rec.Id).substring(0, 15);
String id18 = String.valueOf(rec.Id);
try {
    update new Edge_Rec__A____c(Id = rec.Id, Name = 'r1');
    obs = 'no-error|' + EdgeDepth__A__.n;
} catch (Exception e) {
    obs = e.getTypeName() + '|' + e.getMessage().replace(id18, '<id>').replace(id15, '<id>') + '|depth=' + EdgeDepth__A__.n;
}
