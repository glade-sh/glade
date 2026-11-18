trigger EdgeRecTrg__A__ on Edge_Rec__A____c (after update) {
    EdgeDepth__A__.n++;
    List<Edge_Rec__A____c> up = new List<Edge_Rec__A____c>();
    for (Edge_Rec__A____c r : Trigger.new) { up.add(new Edge_Rec__A____c(Id = r.Id, Name = r.Name + 'x')); }
    update up;
}
