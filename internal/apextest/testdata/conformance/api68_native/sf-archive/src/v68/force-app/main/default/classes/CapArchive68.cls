public class CapArchive68 {
    public static void run() {
        SF_Archive.ArchiverAccessorResponse r =
            SF_Archive.ArchiverAccessor.getMaskingStatus('S68-FIXTURE');
        System.debug(LoggingLevel.ERROR, 'RESPONSE=' + (r != null));
    }
}
