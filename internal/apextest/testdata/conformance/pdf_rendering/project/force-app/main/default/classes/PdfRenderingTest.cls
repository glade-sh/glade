@IsTest private class PdfRenderingTest {
    @IsTest static void text() { System.assert(false, 'CAPTURE|'+JSON.serialize(PdfRenderingProbe.observe('text'))); }
    @IsTest static void nullInput() { System.assert(false, 'CAPTURE|'+JSON.serialize(PdfRenderingProbe.observe('nullInput'))); }
    @IsTest static void emptyInput() { System.assert(false, 'CAPTURE|'+JSON.serialize(PdfRenderingProbe.observe('emptyInput'))); }
    @IsTest static void megabyte() { System.assert(false, 'CAPTURE|'+JSON.serialize(PdfRenderingProbe.observe('megabyte'))); }
    @IsTest static void megabytePlus() { System.assert(false, 'CAPTURE|'+JSON.serialize(PdfRenderingProbe.observe('megabytePlus'))); }
    @IsTest static void page() { System.assert(false, 'CAPTURE|'+JSON.serialize(PdfRenderingProbe.observe('page'))); }
    @IsTest static void missingPage() { System.assert(false, 'CAPTURE|'+JSON.serialize(PdfRenderingProbe.observe('missingPage'))); }
}
