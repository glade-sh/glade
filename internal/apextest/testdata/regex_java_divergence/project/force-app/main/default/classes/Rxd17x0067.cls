/**
 * Rxd17x0067 - Java Pattern COMMENTS mode (?x) predictions for Apex capture.
 *
 * 32 independent @IsTest methods, one per prediction id in PREDICTIONS.tsv.
 * Every flag is inline in the pattern text ((?x), (?dx), (?xU), (?-x), ...), so
 * Pattern.compile is called with one argument. Patterns and subjects are built
 * with ch(<codepoint>) for control characters and for every backslash, so the
 * Apex source has no ambiguous string escapes. Each method compiles the pattern,
 * runs matches() and a fresh find(), and emits one compact payload through
 * System.assert(false, payload). A construction failure is reported as
 * buildError for that row.
 *
 * This class is for a later native capture. It has not been run or deployed.
 */
@IsTest
private class Rxd17x0067 {

    /** One character from its Unicode code point. */
    static String ch(Integer cp) {
        return String.fromCharArray(new List<Integer>{ cp });
    }

    /** Replace control characters so the payload stays safe inside a test message. */
    static String safe(String s) {
        if (s == null) {
            return '';
        }
        String r = '';
        for (Integer i = 0; i < s.length(); i++) {
            Integer cp = s.codePointAt(i);
            if (cp > 65535) {
                r += s.substring(i, i + 2);
                i++;
            } else if (cp < 32 || (cp >= 127 && cp <= 159)) {
                r += '?';
            } else {
                r += String.fromCharArray(new List<Integer>{ cp });
            }
        }
        return r;
    }

    /** Keep a message one key=value field: null-safe and semicolon-free. */
    static String msgText(String s) {
        if (s == null) {
            return '';
        }
        return s.replace(';', ',');
    }

    /** One-line exception text for the payload. */
    static String msg(Exception e) {
        String m = e.getMessage();
        if (m == null) {
            return '';
        }
        return m.replace(ch(10), ' ').replace(ch(13), ' ').replace(';', ',');
    }

    /** Keep an exception message on one line and safe for key=value parsing. */
    static String clean(String s) {
        if (s == null) {
            return '';
        }
        return s.replace(';', ',').replace(ch(10), ' ').replace(ch(13), ' ');
    }

    /** Compile, matches(), fresh find(); return one compact key=value payload. */
    static String payload(String caseId, String pat, String subj) {
        String compileStatus = 'ok';
        String exType = '';
        String exMsg = '';
        String matches = '-';
        String find = '-';
        String start = '';
        String endIdx = '';
        String g0 = '';
        String g0hex = '';
        String g1 = '';
        String capErr = '';
        try {
            Pattern p = Pattern.compile(pat);
            try {
                matches = String.valueOf(p.matcher(subj).matches());
            } catch (Exception e) {
                matches = 'throw';
                exType = e.getTypeName();
                exMsg = safe(e.getMessage()).replace(';', ',');
            }
            try {
                Matcher m = p.matcher(subj);
                Boolean found = m.find();
                find = String.valueOf(found);
                if (found) {
                    try {
                        start = String.valueOf(m.start());
                        endIdx = String.valueOf(m.end());
                        g0 = safe(m.group());
                        try {
                            g0hex = EncodingUtil.convertToHex(Blob.valueOf(m.group()));
                        } catch (Exception h) {
                            g0hex = 'hexError:' + h.getTypeName();
                        }
                        if (m.groupCount() >= 1) {
                            g1 = safe(m.group(1));
                        }
                    } catch (Exception c) {
                        capErr = c.getTypeName() + ':' + safe(c.getMessage()).replace(';', ',');
                    }
                }
            } catch (Exception e) {
                find = 'throw';
                exType = e.getTypeName();
                exMsg = safe(e.getMessage()).replace(';', ',');
            }
        } catch (Exception e) {
            compileStatus = 'error';
            matches = 'ERROR';
            find = 'ERROR';
            exType = e.getTypeName();
            try {
                exMsg = safe(e.getMessage()).replace(';', ',');
            } catch (Exception m) {
                exMsg = 'safeError:' + m.getTypeName();
            }
        }
        return 'id=' + caseId
            + ';compile=' + compileStatus
            + ';type=' + exType
            + ';msg=' + exMsg
            + ';matches=' + matches
            + ';find=' + find
            + ';start=' + start
            + ';end=' + endIdx
            + ';g0=' + g0
            + ';g0hex=' + g0hex
            + ';g1=' + g1
            + ';capErr=' + capErr;
    }

    @IsTest static void d17x_C01() {
        String caseId = 'C01';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)a ' + ch(9) + ch(10) + ch(11) + ch(12) + ch(13) + 'b';
            s = '!ab!';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C02() {
        String caseId = 'C02';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?xU)a' + ch(160) + 'b';
            s = 'a' + ch(160) + 'b';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C03() {
        String caseId = 'C03';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)a' + ch(92) + ' ' + ch(92) + '#b';
            s = 'a #b';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C04() {
        String caseId = 'C04';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)( ?: a )';
            s = 'a';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C05() {
        String caseId = 'C05';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(? x)a';
            s = 'a';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C06() {
        String caseId = 'C06';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x )a';
            s = 'a';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C07() {
        String caseId = 'C07';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)' + ch(92) + '0 1 4';
            s = ch(12);
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C08() {
        String caseId = 'C08';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)' + ch(92) + 'x2 3';
            s = '#';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C09() {
        String caseId = 'C09';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)' + ch(92) + 'u00 41';
            s = 'A';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C10() {
        String caseId = 'C10';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)' + ch(92) + ' 0 1 4';
            s = ' 014';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C11() {
        String caseId = 'C11';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)a{2, 3}';
            s = 'aaaa';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C12() {
        String caseId = 'C12';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)a{ 2,3}';
            s = 'aa';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C13() {
        String caseId = 'C13';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)a{1 2}';
            s = 'aaaaaaaaaaaa';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C14() {
        String caseId = 'C14';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)' + ch(92) + 'x2#c' + ch(10) + '3';
            s = '#';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C15() {
        String caseId = 'C15';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)[a b]';
            s = ' ';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C16() {
        String caseId = 'C16';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)[' + ch(92) + ' ' + ch(92) + '#]';
            s = '#';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C17() {
        String caseId = 'C17';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)[a#c' + ch(10) + 'b]';
            s = 'b';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C18() {
        String caseId = 'C18';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)[#]';
            s = '#';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C19() {
        String caseId = 'C19';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)a#c' + ch(10) + 'b';
            s = 'ab';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C20() {
        String caseId = 'C20';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)a#c' + ch(13) + 'b';
            s = 'ab';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C21() {
        String caseId = 'C21';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?dx)a#c' + ch(13) + 'b';
            s = 'ab';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C22() {
        String caseId = 'C22';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)a#c' + ch(133) + 'b';
            s = 'ab';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C23() {
        String caseId = 'C23';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?dx)a#c' + ch(0) + 'b';
            s = 'a' + ch(0) + 'b';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C24() {
        String caseId = 'C24';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)a#c' + ch(12) + 'b';
            s = 'ab';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C25() {
        String caseId = 'C25';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)a#c' + ch(92) + 'nb';
            s = 'ab';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C26() {
        String caseId = 'C26';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)' + ch(92) + 'Q a # b ' + ch(92) + 'E c';
            s = ' a # b c';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C27() {
        String caseId = 'C27';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)#' + ch(92) + 'Q' + ch(10) + ' a ' + ch(92) + 'E';
            s = ' a ';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C28() {
        String caseId = 'C28';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x:a b) c';
            s = 'ab c';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C29() {
        String caseId = 'C29';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)a (?-x) b #';
            s = 'a b #';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C30() {
        String caseId = 'C30';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)(a(?-x) b ) c';
            s = 'a b c';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C31() {
        String caseId = 'C31';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = 'a (?x)b';
            s = 'a b';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }

    @IsTest static void d17x_C32() {
        String caseId = 'C32';
        String p = '';
        String s = '';
        String buildErr = '';
        try {
            p = '(?x)(?-x )a';
            s = 'a';
        } catch (Exception e) {
            buildErr = e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',');
        }
        if (buildErr != '') {
            System.assert(false, 'id=' + caseId + ';buildError=' + buildErr
                + ';matches=-;find=-;start=;end=;g0=;g1=');
        } else {
            String out = '';
            try {
                out = payload(caseId, p, s);
            } catch (Exception e) {
                out = 'id=' + caseId + ';payloadError=' + e.getTypeName() + ':' + safe(e.getMessage()).replace(';', ',')
                    + ';matches=-;find=-;start=;end=;g0=;g1=';
            }
            System.assert(false, out);
        }
    }
}
