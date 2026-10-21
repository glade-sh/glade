global class UrlRewriterProbe implements Site.UrlRewriter {
    global PageReference mapRequestUrl(PageReference friendlyUrl) {
        return new PageReference(friendlyUrl.getUrl() + '?mapped=1');
    }
    global List<PageReference> generateUrlFor(List<PageReference> salesforceUrls) {
        List<PageReference> r = new List<PageReference>();
        for (PageReference p : salesforceUrls) { r.add(new PageReference('/f' + p.getUrl())); }
        return r;
    }
}
