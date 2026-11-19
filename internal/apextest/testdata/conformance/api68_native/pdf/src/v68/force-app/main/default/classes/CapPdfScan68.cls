public class CapPdfScan68 {
    public static void run() {
        try {
            Blob pdf = Page.CapPdf68.getContentAsPDF();
            ContentVersion cv = new ContentVersion(Title = 'cap68-pdf',
                PathOnClient = 'cap68.pdf', VersionData = pdf);
            insert cv;
            System.debug(LoggingLevel.ERROR, 'PDF_BYTES=' + pdf.size() + ';SAVED=true');
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'ERR=' + e.getTypeName() + ': ' + e.getMessage());
        }
    }
}
