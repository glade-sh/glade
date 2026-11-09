// N6: evaluate the comparisons in a fresh transaction (no DML). Run after
// n6_same_txn (locale now tr_TR) and again after n6_restore (en_US).
String upperI = String.fromCharArray(new List<Integer>{73});
String lowerI = String.fromCharArray(new List<Integer>{105});
String dotted = String.fromCharArray(new List<Integer>{304});
String fi = String.fromCharArray(new List<Integer>{64257});
String fiAscii = String.fromCharArray(new List<Integer>{102,105});
Map<String,Object> m = new Map<String,Object>();
m.put('I_i', upperI == lowerI);
m.put('dotted_i', dotted == lowerI);
m.put('fi_lig_fi', fi == fiAscii);
m.put('I_i_eqic', upperI.equalsIgnoreCase(lowerI));
m.put('i_upper', lowerI.toUpperCase().getChars());
m.put('locale', UserInfo.getLocale());
m.put('queried', [SELECT LocaleSidKey FROM User WHERE Id = :UserInfo.getUserId()].LocaleSidKey);
System.debug('CAP|n6.p1.new_txn|' + JSON.serialize(m));
