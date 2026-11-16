Edge_RecM__A____c rec = new Edge_RecM__A____c(Name = 'r'); insert rec;
EdgeDepthM__A__.n = 0; EdgeDepthM__A__.log = ''; EdgeDepthM__A__.mode = 'plain';
try {
    update new Edge_RecM__A____c(Id = rec.Id, Name = 'r1');
    obs = 'no-error|depth=' + EdgeDepthM__A__.n + '|' + EdgeDepthM__A__.log;
} catch (Exception e) {
    obs = e.getTypeName() + '|depth=' + EdgeDepthM__A__.n + '|' + EdgeDepthM__A__.log + '|' + EdgeDepthM__A__.scrub(e.getMessage(), rec.Id).left(400);
}
