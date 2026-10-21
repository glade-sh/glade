public virtual class NamedDriver extends DriverHost.BaseDriver {
    public virtual override String run(Request value) { return 'count:' + value.count; }
    public String createRequest() { Request value = new Request(); return run(value); }
}
