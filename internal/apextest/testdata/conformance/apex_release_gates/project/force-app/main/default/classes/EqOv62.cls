public class EqOv62 {
  public class EqException extends Exception {
    public Integer k=0;
    public override Boolean equals(Object o){ return o instanceof EqException && ((EqException)o).k==this.k; }
    public override Integer hashCode(){ return k; }
  }
}
