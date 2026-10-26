@IsTest
private class PdfProbeTest {
    @IsTest static void probeA() {
        PdfProbe.compact = true;
        System.assert(false, 'TOPDF_TEST_ROUTE:' + PdfProbe.run('A'));
    }
    @IsTest static void probeB() {
        PdfProbe.compact = true;
        System.assert(false, 'TOPDF_TEST_ROUTE:' + PdfProbe.run('B'));
    }
    @IsTest static void pages() {
        System.assert(false, 'PAGES_TEST_ROUTE:' + PdfProbe.runPages());
    }
    @IsTest static void sample() {
        System.assert(false, 'SAMPLE_TEST_ROUTE:' + PdfProbe.sampleB64('<html><body><p>Hello PDF world</p></body></html>'));
    }
}
