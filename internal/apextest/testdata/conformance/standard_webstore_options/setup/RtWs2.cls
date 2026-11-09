@IsTest
private class RtWs2 {
    @IsTest static void preserve() {
        List<String> out = new List<String>();
        for (String mode : new List<String>{'unset','false','true'}) {
            for (Boolean guest : new List<Boolean>{false, true}) {
                SObject s = Schema.getGlobalDescribe().get('WebStore').newSObject();
                s.put('Name', 'p_' + mode + '_' + guest);
                s.put('OptionsGuestCartEnabled', guest);
                if (mode != 'unset') s.put('OptionsPreserveGuestCartEnabled', mode == 'true');
                insert s;
                Boolean afterInsert = (Boolean)[SELECT OptionsPreserveGuestCartEnabled FROM WebStore WHERE Id = :s.Id].OptionsPreserveGuestCartEnabled;
                s.put('OptionsPreserveGuestCartEnabled', false);
                update s;
                Boolean afterUpdate = (Boolean)[SELECT OptionsPreserveGuestCartEnabled FROM WebStore WHERE Id = :s.Id].OptionsPreserveGuestCartEnabled;
                out.add('preserve_' + mode + '_guest' + guest + '=insert ' + afterInsert + ',update_false ' + afterUpdate);
            }
        }
        System.assert(false, String.join(out, ' ;; '));
    }
}
