// N6 probe 1: admin (en_US) evaluates String == before and after a DML update of
// its own LocaleSidKey in the SAME transaction. Operands use fromCharArray so no
// compile-time literal folding can hide the runtime comparison.
String upperI = String.fromCharArray(new List<Integer>{73});
String lowerI = String.fromCharArray(new List<Integer>{105});
String dotted = String.fromCharArray(new List<Integer>{304});
String fi = String.fromCharArray(new List<Integer>{64257});
String fiAscii = String.fromCharArray(new List<Integer>{102,105});
Map<String,Object> step(String name) {
    Map<String,Object> m = new Map<String,Object>();
    m.put('I_i', upperI == lowerI);
    m.put('dotted_i', dotted == lowerI);
    m.put('fi_lig_fi', fi == fiAscii);
    m.put('I_i_eqic', upperI.equalsIgnoreCase(lowerI));
    m.put('i_upper', lowerI.toUpperCase().getChars());
    m.put('locale', UserInfo.getLocale());
    m.put('queried', [SELECT LocaleSidKey FROM User WHERE Id = :UserInfo.getUserId()].LocaleSidKey);
    System.debug('CAP|n6.p1.' + name + '|' + JSON.serialize(m));
    return m;
}
step('before');
User me = new User(Id = UserInfo.getUserId(), LocaleSidKey = 'tr_TR');
update me;
step('after_update');
User me2 = [SELECT Id, LocaleSidKey FROM User WHERE Id = :UserInfo.getUserId()];
step('after_requery');
