@IsTest
private class RelProbeTest {
    private static void obs(String key) {
        List<String> out = new List<String>();
        for (String t : new List<String>{'Customer','Other'}) { out.add(t + '|' + RelProbe.run(key, t, t)); }
        System.assert(false, 'ROWS\n' + String.join(out, '\n'));
    }
    @IsTest static void tD1() { obs('D1'); }
    @IsTest static void tD2() { obs('D2'); }
    @IsTest static void tD3() { obs('D3'); }
    @IsTest static void tD4() { obs('D4'); }
    @IsTest static void tD5() { obs('D5'); }
    @IsTest static void tD6() { obs('D6'); }
    @IsTest static void tD7() { obs('D7'); }
    @IsTest static void tD8() { obs('D8'); }
    @IsTest static void tD9() { obs('D9'); }
    @IsTest static void tL1() { obs('L1'); }
    @IsTest static void tL2() { obs('L2'); }
    @IsTest static void tV1() { obs('V1'); }
    @IsTest static void tV2() { obs('V2'); }
    @IsTest static void tC1() { obs('C1'); }
    @IsTest static void tRO1() { obs('RO1'); }
    @IsTest static void tRO2() { obs('RO2'); }
    @IsTest static void tRO3() { obs('RO3'); }
    @IsTest static void tCR1() { obs('CR1'); }
    @IsTest static void tCR2() { obs('CR2'); }
    @IsTest static void tCR3() { obs('CR3'); }
    @IsTest static void tEM1() { obs('EM1'); }
    @IsTest static void tEM2() { obs('EM2'); }
    @IsTest static void tEM3() { obs('EM3'); }
}
