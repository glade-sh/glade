public class SetBufferProbe {
    public static String run(String route) {
        List<Id> input = new List<Id>();
        for (Integer i = 0; i < 64; i++) input.add((Id)('001' + String.valueOf(i).leftPad(12, '0')));
        Set<Id> values = new Set<Id>();
        if (route == 'add') {
            for (Id item : input) values.add(item);
        } else if (route == 'addAll_list') {
            values.addAll(input);
        } else if (route == 'addAll_set') {
            values.addAll(new Set<Id>(input));

        } else if (route == 'literal') {
            values = new Set<Id>{'001000000000000', '001000000000001', '001000000000002', '001000000000003', '001000000000004', '001000000000005', '001000000000006', '001000000000007', '001000000000008', '001000000000009', '001000000000010', '001000000000011', '001000000000012', '001000000000013', '001000000000014', '001000000000015', '001000000000016', '001000000000017', '001000000000018', '001000000000019', '001000000000020', '001000000000021', '001000000000022', '001000000000023', '001000000000024', '001000000000025', '001000000000026', '001000000000027', '001000000000028', '001000000000029', '001000000000030', '001000000000031', '001000000000032', '001000000000033', '001000000000034', '001000000000035', '001000000000036', '001000000000037', '001000000000038', '001000000000039', '001000000000040', '001000000000041', '001000000000042', '001000000000043', '001000000000044', '001000000000045', '001000000000046', '001000000000047', '001000000000048', '001000000000049', '001000000000050', '001000000000051', '001000000000052', '001000000000053', '001000000000054', '001000000000055', '001000000000056', '001000000000057', '001000000000058', '001000000000059', '001000000000060', '001000000000061', '001000000000062', '001000000000063'};
        } else if (route == 'list_constructor') {
            values = new Set<Id>(input);
        } else if (route == 'set_constructor') {
            values = new Set<Id>(new Set<Id>(input));
        } else {
            System.assert(false, route);
        }
        Set<Id> alias = values;
        Set<Id> copy = values.clone();
        Boolean duplicate = values.add((Id)'001000000000001');
        Boolean inserted = values.add((Id)'001000000000064');
        return values.size() + ':' + alias.size() + ':' + copy.size() + ':' +
            duplicate + ':' + inserted + ':' + values.contains((Id)'001000000000063') + ':' + copy.contains((Id)'001000000000064');
    }
    public static Map<String,String> observe() {
        Map<String,String> result = new Map<String,String>();
        for (String route : new List<String>{'add','addAll_list','addAll_set','literal','list_constructor','set_constructor'}) {
            result.put(route, run(route));
        }
        return result;
    }
}
