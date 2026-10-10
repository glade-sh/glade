@IsTest
private class PageLookupProbe {
    private static String pageName(String address) {
        return address.replaceFirst('(?i)^/apex/(?:[A-Za-z][A-Za-z0-9_]*__)?', '');
    }

    private static List<SObject> lookup(Set<String> selectedNames) {
        return Database.query('SELECT Id, Name FROM ApexPage WHERE Name IN :selectedNames');
    }

    @IsTest static void tokenUrlAndPrefix() {
        String address = Page.ProbeLanding.getUrl();
        System.assertEquals('/apex/ProbeLanding', address, 'Page token URL');
        System.assertEquals('ProbeLanding', pageName(address), 'Token prefix removal');
        System.assertEquals('ProbeLanding', pageName('/APEX/probe__ProbeLanding'), 'Namespaced prefix control');
    }

    @IsTest static void directSetFindsPage() {
        Set<String> selectedNames = new Set<String>{'ProbeLanding'};
        List<SObject> rows = Database.query('SELECT Id, Name FROM ApexPage WHERE Name IN :selectedNames');
        System.assertEquals(1, rows.size(), 'Direct typed Set bind must find loaded page');
        System.assertEquals('ProbeLanding', rows[0].get('Name'));
    }

    @IsTest static void tokenNameFindsPageAcrossCall() {
        Set<String> selectedNames = new Set<String>{pageName(Page.ProbeLanding.getUrl())};
        List<SObject> rows = lookup(selectedNames);
        System.assertEquals(1, rows.size(), 'Token-derived page name and typed parameter');
        System.assertEquals('ProbeLanding', rows[0].get('Name'));
    }

    @IsTest static void missingPageReturnsEmpty() {
        List<SObject> rows = lookup(new Set<String>{'ProbeMissing'});
        System.assertEquals(0, rows.size(), 'A name without a page must not match');
    }

    @IsTest static void literalQueryFindsMetadata() {
        List<ApexPage> rows = [SELECT Id, Name, NamespacePrefix FROM ApexPage WHERE Name = 'ProbeLanding'];
        System.assertEquals(1, rows.size(), 'Independent literal-query metadata control');
        System.assertEquals('ProbeLanding', rows[0].Name);
    }
}
