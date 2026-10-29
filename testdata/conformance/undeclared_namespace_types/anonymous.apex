Object made = BarConstructions.make(false);
Boolean live = false;
if (live) {
    Object wrapper = new bar.Wrapper();
    Object others = new List<bar.OtherWrapper>();
    Object owned = new fooNs.LocalThing();
    Object absent = new fooNs.Unknown();
    System.debug(new bar.LogBuilder());
}
System.debug(made);
