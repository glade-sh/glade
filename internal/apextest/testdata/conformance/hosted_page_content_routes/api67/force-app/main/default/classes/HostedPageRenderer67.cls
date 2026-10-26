public class HostedPageRenderer67 {
    public static Blob render(PageReference pageRef, Boolean asPdf) {
        if (asPdf) { return pageRef.getContentAsPDF(); }
        return pageRef.getContent();
    }
}
