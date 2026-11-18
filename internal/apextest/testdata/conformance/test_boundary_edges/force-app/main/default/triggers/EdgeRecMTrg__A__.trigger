trigger EdgeRecMTrg__A__ on Edge_RecM__A____c (after update) {
    EdgeDepthM__A__.n++;
    Integer d = EdgeDepthM__A__.n;
    List<Edge_RecM__A____c> up = new List<Edge_RecM__A____c>();
    for (Edge_RecM__A____c r : Trigger.new) { up.add(new Edge_RecM__A____c(Id = r.Id, Name = 'm' + d)); }
    if (EdgeDepthM__A__.mode == 'partial') {
        List<Database.SaveResult> rs = Database.update(up, false);
        if (!rs[0].isSuccess()) {
            EdgeDepthM__A__.log += d + ':' + rs[0].getErrors()[0].getStatusCode() + ':' + EdgeDepthM__A__.scrub(rs[0].getErrors()[0].getMessage(), up[0].Id).left(70) + ';';
        }
    } else if (EdgeDepthM__A__.mode == 'catch') {
        try { update up; } catch (Exception e) {
            EdgeDepthM__A__.log += d + ':' + e.getTypeName() + ':' + EdgeDepthM__A__.scrub(e.getMessage(), up[0].Id).left(150) + ';';
        }
    } else {
        update up;
    }
}
