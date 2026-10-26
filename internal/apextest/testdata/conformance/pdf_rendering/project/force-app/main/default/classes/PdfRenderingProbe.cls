public class PdfRenderingProbe {
    public static Object observe(String shape) {
        Map<String,Object> out = new Map<String,Object>();
        try {
            Blob b;
            if (shape == 'page') b = new PageReference('/apex/CapturePdfPage').getContentAsPDF();
            else if (shape == 'missingPage') b = new PageReference('/apex/CaptureAbsentPage').getContentAsPDF();
            else {
                String input;
                if (shape == 'text') input = '<p>x</p>';
                else if (shape == 'emptyInput') input = '';
                else if (shape == 'megabyte') input = 'x'.repeat(1048576);
                else if (shape == 'megabytePlus') input = 'x'.repeat(1048577);
                b = Blob.toPdf(input);
            }
            String encoded = EncodingUtil.base64Encode(b);
            out.put('positive', b.size() > 0);
            out.put('first5', EncodingUtil.base64Decode(encoded.left(8)).toString().left(5));
            out.put('base64_8', encoded.left(8));
        } catch (Exception e) {
            out.put('exceptionType', e.getTypeName());
            out.put('exceptionMessage', e.getMessage());
        }
        return out;
    }
}
