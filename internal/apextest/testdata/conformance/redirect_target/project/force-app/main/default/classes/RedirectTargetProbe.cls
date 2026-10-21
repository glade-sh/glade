public class RedirectTargetProbe {
    public static Map<String, String> observe() {
        Map<String, String> out = new Map<String, String>();
        for (String key : new List<String>{'null', 'relative', 'absolute', 'login'}) {
            try {
                PageReference target;
                if (key == 'relative') target = new PageReference('/example');
                if (key == 'absolute') target = new PageReference('https://example.invalid/path');
                if (key == 'login') target = Site.login('unregistered@example.invalid', 'unused', null);
                Aura.redirect(target);
                out.put(key, 'accepted');
            } catch (Exception e) {
                out.put(key, e.getTypeName() + ': ' + e.getMessage());
            }
        }
        return out;
    }
}
