@IsTest
private class R_S01 {
    @IsTest static void t() {
        String r = String.valueOf(new gladens004.V035Provider.GlobalInner().size());
        System.assert(false, 'OBS|S01|' + r);
    }
}
