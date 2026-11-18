public class CapDcLimits68 {
    public static void run() {
        System.debug(LoggingLevel.ERROR, 'CURSORS=' + Limits.getDataCloudCursors()
            + ';CURSOR_LIMIT=' + Limits.getLimitDataCloudCursors()
            + ';ROWS=' + Limits.getDataCloudCursorRows()
            + ';ROW_LIMIT=' + Limits.getLimitDataCloudCursorRows());
    }
}
