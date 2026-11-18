public class CapPdfScan67 {
    public static void run() {
        try {
            Blob pdf = Page.CapPdf67.getContentAsPDF();
            ContentVersion cv = new ContentVersion(Title = 'cap67-pdf',
                PathOnClient = 'cap67.pdf', VersionData = pdf);
            insert cv;
            System.debug(LoggingLevel.ERROR, 'PDF_BYTES=' + pdf.size() + ';SAVED=true');
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'ERR=' + e.getTypeName() + ': ' + e.getMessage());
        }
    }
}
