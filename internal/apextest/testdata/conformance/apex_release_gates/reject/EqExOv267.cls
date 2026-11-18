public virtual class EqExOv267 {
  public virtual class BaseException extends Exception { }
  public class KidException extends BaseException { public override String getMessage(){ return 'x'; } public override Boolean equals(Object o){ return true; } public override Integer hashCode(){ return 1; } }
  public class StrException extends Exception { public override String toString(){ return 't'; } }
  public class DrException extends Exception { public override Boolean doesNotExist(Object o){ return true; } }
}
