@isTest
private class VMExceptionDispatchTest {
  private static String trail = '';

  private interface Invoker {
    void run();
  }

  private class ThrowingInvoker implements Invoker {
    public void run() {
      throw new QueryException('bad query');
    }
  }

  private static void callThroughInterface(Invoker invoker) {
    invoker.run();
  }

  private static void catchRethrowAndFinally() {
    try {
      callThroughInterface(new ThrowingInvoker());
    } catch (Exception e) {
      trail += 'inner:' + e.getTypeName() + ';';
      throw e;
    } finally {
      trail += 'finally;';
    }
  }

  @isTest static void interfaceDispatchRethrowFinallyAndExceptionMetadata() {
    try {
      catchRethrowAndFinally();
      System.assert(false, 'expected QueryException');
    } catch (QueryException e) {
      System.assertEquals('System.QueryException', e.getTypeName());
      System.assertEquals('bad query', e.getMessage());
      System.assert(e.getLineNumber() > 0);
      System.assert(e.getStackTraceString().contains('ThrowingInvoker.run'));
      System.assert(trail.contains('inner:System.QueryException;'));
      System.assert(trail.contains('finally;'));
    } catch (Exception e) {
      System.assert(false, 'wrong exception type: ' + e.getTypeName());
    }
  }
}
