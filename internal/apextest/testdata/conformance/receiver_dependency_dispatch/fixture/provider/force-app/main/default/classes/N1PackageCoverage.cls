@IsTest
private class N1PackageCoverage {
    @IsTest
    static void coversOwnedPackageClasses() {
        Builder b = new Builder();
        List<String> seen = new List<String>();
        seen.addAll(b.insertRecords(1));
        seen.addAll(b.direct(1));
        seen.addAll(b.chained(1));
        seen.addAll(b.thisQualified(1));
        seen.addAll(b.superQualified(1));
        seen.add(b.self().pkgOnly());
        seen.add(b.classApi());
        seen.add(b.globalToken());
        seen.add(b.privateProof());
        seen.add(BuilderBase.staticToken());
        seen.add(String.valueOf(Caller.run()));
        seen.add(Caller.classApi());
        seen.add(N1Api67.classApi() + N1Api67.token());
        seen.add(N1Api68.classApi() + N1Api68.token());
        Item i = new Item();
        i.name = 'covered';
        Data d = new Data();
        d.items = new List<Item>{ i };
        seen.add(String.valueOf(Util.has(new List<Id>())));
        seen.add(String.valueOf(Util.has(new List<String>{ 'x' })));
        seen.add(String.valueOf(Util.has(d.items)));
        System.assertEquals(17, seen.size());
    }
}
