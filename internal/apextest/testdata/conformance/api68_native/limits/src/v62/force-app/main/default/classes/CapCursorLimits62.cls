public class CapCursorLimits62 {
    public static void run() {
        logCounters('BEFORE');
        try {
            Database.Cursor curs = Database.getCursor('SELECT Id FROM Account LIMIT 5');
            System.debug(LoggingLevel.ERROR, 'NUM=' + curs.getNumRecords());
            try {
                System.debug(LoggingLevel.ERROR, 'FETCH1=' + curs.fetch(0, 1).size());
            } catch (Exception fe) {
                System.debug(LoggingLevel.ERROR, 'FETCH1_ERR=' + fe.getTypeName() + ': ' + fe.getMessage());
            }
            try {
                System.debug(LoggingLevel.ERROR, 'FETCH_PAST=' + curs.fetch(5, 1).size());
            } catch (Exception pe) {
                System.debug(LoggingLevel.ERROR, 'FETCH_PAST_ERR=' + pe.getTypeName() + ': ' + pe.getMessage());
            }
        } catch (Exception ce) {
            System.debug(LoggingLevel.ERROR, 'CURSOR_ERR=' + ce.getTypeName() + ': ' + ce.getMessage());
        }
        logCounters('AFTER');
    }
    public static void logCounters(String tag) {
        System.debug(LoggingLevel.ERROR, tag
            + ';APEX_CURSORS=' + Limits.getApexCursors()
            + ';APEX_CURSOR_LIMIT=' + Limits.getLimitApexCursors()
            + ';PAGINATION_CURSORS=' + Limits.getApexPaginationCursors()
            + ';PAGINATION_CURSOR_LIMIT=' + Limits.getLimitApexPaginationCursors()
            + ';CURSOR_ROWS=' + Limits.getApexCursorRows()
            + ';CURSOR_ROW_LIMIT=' + Limits.getLimitApexCursorRows()
            + ';PAGINATION_ROWS=' + Limits.getApexPaginationCursorRows()
            + ';PAGINATION_ROW_LIMIT=' + Limits.getLimitApexPaginationCursorRows());
    }
}
