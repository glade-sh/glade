global class Q3Act {
    global class Req { @InvocableVariable global String name; }
    global class Res { @InvocableVariable global String msg; @InvocableVariable global Boolean ok; }
    @InvocableMethod(label='Q3 Act')
    global static List<Res> go(List<Req> reqs) {
        List<Res> out = new List<Res>();
        for (Req r : reqs) { Res x = new Res(); x.msg = 'hi ' + r.name; x.ok = true; out.add(x); }
        return out;
    }
}
