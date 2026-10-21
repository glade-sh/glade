@IsTest private class StubDispatchCases {
    // Capture candidates only. Expected observations remain '?' in cases.json.
    private static void observe(String id) {
        System.debug('ROW|' + id + '|' + StubDispatchProbe.run(id));
    }
    @IsTest static void A01() { observe('A01'); }
    @IsTest static void A02() { observe('A02'); }
    @IsTest static void A03() { observe('A03'); }
    @IsTest static void A04() { observe('A04'); }
    @IsTest static void H01() { observe('H01'); }
    @IsTest static void H02() { observe('H02'); }
    @IsTest static void H03() { observe('H03'); }
    @IsTest static void H04() { observe('H04'); }
    @IsTest static void H05() { observe('H05'); }
    @IsTest static void H06() { observe('H06'); }
    @IsTest static void H07() { observe('H07'); }
    @IsTest static void H08() { observe('H08'); }
    @IsTest static void D01() { observe('D01'); }
    @IsTest static void D02() { observe('D02'); }
    @IsTest static void D03() { observe('D03'); }
    @IsTest static void D04() { observe('D04'); }
    @IsTest static void D05() { observe('D05'); }
    @IsTest static void D06() { observe('D06'); }
    @IsTest static void D07() { observe('D07'); }
    @IsTest static void D08() { observe('D08'); }
    @IsTest static void D09() { observe('D09'); }
    @IsTest static void L01() { observe('L01'); }
    @IsTest static void L02() { observe('L02'); }
    @IsTest static void L03() { observe('L03'); }
    @IsTest static void L04() { observe('L04'); }
    @IsTest static void L05() { observe('L05'); }
    @IsTest static void L06() { observe('L06'); }
    @IsTest static void E01() { observe('E01'); }
    @IsTest static void E02() { observe('E02'); }
    @IsTest static void E03() { observe('E03'); }
    @IsTest static void E04() { observe('E04'); }
    @IsTest static void E05() { observe('E05'); }
    @IsTest static void E06() { observe('E06'); }
    @IsTest static void E07() { observe('E07'); }
    @IsTest static void E08() { observe('E08'); }
    @IsTest static void E09() { observe('E09'); }
}
