public class LocalCallable implements System.Callable {
  public Object call(String action, Map<String, Object> args) {
    if (action == 'echo') {
      return args.get('value');
    }
    return action;
  }
}
