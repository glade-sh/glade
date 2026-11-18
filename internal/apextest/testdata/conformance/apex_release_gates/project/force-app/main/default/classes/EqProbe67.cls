public class EqProbe67 {
  public class EqException extends Exception {
    public Integer k=0;
    public Boolean equals(Object o){ return o instanceof EqException && ((EqException)o).k==this.k; }
    public Integer hashCode(){ return k; }
  }
  public class Plain { public Integer k=0; public Boolean equals(Object o){ return o instanceof Plain && ((Plain)o).k==this.k; } public Integer hashCode(){ return k; } }
  public static String run(){
    List<String> o=new List<String>();
    EqException a=new EqException('l'); EqException b=new EqException('r');
    o.add('EQ2 eq='+a.equals(b)+' set='+new Set<Object>{a,b}.size()+' ==:'+(a==b)+' hash='+a.hashCode());
    Map<Object,Integer> m=new Map<Object,Integer>{a=>1};
    o.add('EQ3 contains='+m.containsKey(b));
    Plain p=new Plain(), q=new Plain();
    o.add('EQ5 eq='+p.equals(q)+' set='+new Set<Object>{p,q}.size()+' ==:'+(p==q)+' hash='+p.hashCode());
    Object oa=a, ob=b;
    o.add('EQ6 objEq='+oa.equals(ob)+' ==:'+(oa==ob)+' listIdx='+new List<Object>{a}.indexOf(b)+' listContains='+new List<Object>{a}.contains(b));
    return String.join(o,' ## ');
  }
}
