@IsTest
private class UseItTest {
    @IsTest static void indexed() { System.assertEquals(7, UseIt.run()); }
    @IsTest static void inherited() {
        Sub s = new Sub(); s.items.add(new HolderA.Item()); s.items[0].parts.add('z');
        System.assertEquals(3, s.go());
        Sub2 t = new Sub2(); t.items.add(new HolderB.Item());
        System.assertEquals(2, t.go());
    }
}
