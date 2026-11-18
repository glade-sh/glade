@IsTest
private class DecHashMapLit {
    @IsTest static void C04() {
        List<String> out = new List<String>();
        Map<Decimal, String> m = new Map<Decimal, String>{ 1.5 => 'a' };
        out.add('C04\tMap\t1.5\tmap_literal\t' + String.valueOf(m.get(1.50)) + '\t\t\t\t'
            + String.valueOf(m.size()) + '\t\t\tmap_literal_compiles');
        out.add('END');
        System.assert(false, '\n' + String.join(out, '\n'));
    }
}
