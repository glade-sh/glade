@IsTest
private class UrlRewriterProbeTest {
    @IsTest static void observe() {
        Map<String,String> o = new Map<String,String>();
        Site.UrlRewriter r = new UrlRewriterProbe();
        o.put('map', r.mapRequestUrl(new PageReference('/x')).getUrl());
        o.put('gen', String.valueOf(r.generateUrlFor(new List<PageReference>{new PageReference('/a'), new PageReference('/b')}).size()) + ':' + r.generateUrlFor(new List<PageReference>{new PageReference('/a')})[0].getUrl());
        Object obj = new UrlRewriterProbe();
        o.put('instanceof', String.valueOf(obj instanceof Site.UrlRewriter));
        Site.UrlRewriter casted = (Site.UrlRewriter) obj;
        o.put('cast', String.valueOf(casted != null));
        try { r.mapRequestUrl(null); o.put('null','ok'); } catch (Exception e) { o.put('null', e.getTypeName()); }
        Assert.fail('OBS|' + JSON.serialize(o));
    }
}
