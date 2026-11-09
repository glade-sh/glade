public class AuraRedirectProbe {
    static String attempt(String label, Object action) { return label; }
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        try { aura.redirect(null); result.put('redirectNull', 'no exception'); } catch (Exception issue) { result.put('redirectNull', issue.getTypeName() + ':' + issue.getMessage()); }
        try { PageReference page = new PageReference('/home'); aura.redirect(page); result.put('redirectPage', 'no exception'); } catch (Exception issue) { result.put('redirectPage', issue.getTypeName() + ':' + issue.getMessage()); }
        try { ApexPages.PageReference login = Site.login('testUser', 'fakepwd', null); result.put('siteLoginNull', login == null ? 'null' : 'page:' + login.getUrl()); } catch (Exception issue) { result.put('siteLoginNull', issue.getTypeName() + ':' + issue.getMessage()); }
        try { ApexPages.PageReference login = Site.login('testUser', 'fakepwd', '/home'); result.put('siteLoginStart', login == null ? 'null' : 'page:' + login.getUrl()); } catch (Exception issue) { result.put('siteLoginStart', issue.getTypeName() + ':' + issue.getMessage()); }
        try { ApexPages.PageReference login = Site.login(null, null, null); result.put('siteLoginAllNull', login == null ? 'null' : 'page:' + login.getUrl()); } catch (Exception issue) { result.put('siteLoginAllNull', issue.getTypeName() + ':' + issue.getMessage()); }
        try { ApexPages.PageReference login = Site.login('testUser', 'fakepwd', null); aura.redirect(login); result.put('loginThenRedirect', 'no exception'); } catch (Exception issue) { result.put('loginThenRedirect', issue.getTypeName() + ':' + issue.getMessage()); }
        return result;
    }
}
