global class TrailService {
  global static String ok() {
    return TrailHelper.internalValue();
  }

  public static String hidden() {
    return 'hidden';
  }
}
