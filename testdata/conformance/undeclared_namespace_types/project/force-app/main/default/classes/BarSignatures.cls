public with sharing class BarSignatures {
    public bar.Wrapper wrapper;
    public List<bar.OtherWrapper> others;
    public Map<String, bar.Wrapper> byKey;
    public fooNs.LocalThing ownType;
    public fooNs.Unknown absentType;
    public BarSignatures() {}
    public BarSignatures(bar.Wrapper initial) { wrapper = initial; }
    public bar.OtherWrapper relay(bar.Wrapper request, List<bar.OtherWrapper> history, fooNs.Unknown extra) {
        return null;
    }
    public Integer guarded(Boolean live) {
        if (!live) { return 0; }
        try {
            return 1;
        } catch (bar.CustomException e) {
            return -1;
        }
    }
}
