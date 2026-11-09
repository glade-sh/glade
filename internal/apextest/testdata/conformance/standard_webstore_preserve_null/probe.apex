for (String mode : new List<String>{'unset', 'false', 'null'}) {
    try {
        WebStore s = new WebStore(Name = 'Capture ' + mode);
        if (mode == 'false') s.put('OptionsPreserveGuestCartEnabled', false);
        if (mode == 'null') s.put('OptionsPreserveGuestCartEnabled', null);
        insert s;
        WebStore result = [SELECT OptionsPreserveGuestCartEnabled FROM WebStore WHERE Id = :s.Id];
        Boolean first = result.OptionsPreserveGuestCartEnabled;
        s.put('OptionsPreserveGuestCartEnabled', false);
        update s;
        result = [SELECT OptionsPreserveGuestCartEnabled FROM WebStore WHERE Id = :s.Id];
        System.debug('C09CAP|' + mode + '|insert=' + first + ',update=' + result.OptionsPreserveGuestCartEnabled);
    } catch (Exception e) {
        System.debug('C09CAP|' + mode + '|error=' + e.getMessage());
    }
}
