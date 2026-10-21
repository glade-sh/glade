public abstract class BaseFailureException extends Exception {
    public override String getStackTraceString() { return 'owned stack'; }
}
