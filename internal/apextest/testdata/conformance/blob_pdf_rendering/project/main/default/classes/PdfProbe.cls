public with sharing class PdfProbe {
    // Returns a JSON object keyed by case id. Each value is a map of facets.
    public static String run() { return run(null); }
    public static String run(String part) {
        Map<String, Object> out = new Map<String, Object>();
        if (part == null || part == 'A') {
        put(out, 'plain_text', 'Hello PDF world');
        try { Blob.toPdf('Hello').toString(); out.put('toString_on_pdf', 'ok'); } catch (Exception e) { out.put('toString_on_pdf', e.getTypeName() + '|' + e.getMessage()); }
        put(out, 'simple_html', '<html><body><h1>Title</h1><p>Hello <b>bold</b> world</p></body></html>');
        put(out, 'css', '<html><head><style>body{font-family:Arial;color:#f00} p{margin:40px}</style></head><body><p>styled</p></body></html>');
        put(out, 'table', '<html><body><table border="1"><tr><th>A</th><th>B</th></tr><tr><td>1</td><td>2</td></tr></table></body></html>');
        put(out, 'img_data_uri', '<html><body><p>before</p><img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="/><p>after</p></body></html>');
        put(out, 'img_external', '<html><body><p>before</p><img src="https://www.salesforce.com/content/dam/web/en_us/www/images/nav/salesforce-logo.svg"/><p>after</p></body></html>');
        put(out, 'img_relative', '<html><body><p>before</p><img src="/img/logo190.png"/><p>after</p></body></html>');
        put(out, 'non_ascii', '<html><body><p>café naïve € 日本語 Здравствуй</p></body></html>');
        put(out, 'empty', '');
        put(out, 'whitespace', '   ');
        put(out, 'not_html', '<<<<not html >>>> & stuff');
        put(out, 'two_pages', '<html><head><style>.pb{page-break-before:always}</style></head><body><p>one</p><div class="pb"></div><p>two</p></body></html>');
        put(out, 'landscape_css', '<html><head><style>@page{size:landscape;margin:0}</style></head><body><p>wide</p></body></html>');
        put(out, 'a4_css', '<html><head><style>@page{size:A4}</style></head><body><p>a4</p></body></html>');
        put(out, 'unicode_font', '<html><head><style>body{font-family:"Arial Unicode MS",sans-serif}</style></head><body><p>caf\u00e9 \u65e5\u672c\u8a9e \u0417\u0434\u0440\u0430\u0432\u0441\u0442\u0432\u0443\u0439</p></body></html>');
        put(out, 'helvetica_font', '<html><head><style>body{font-family:Helvetica,sans-serif}</style></head><body><p>helv</p></body></html>');
        put(out, 'script_tag', '<html><body><script>document.write("js")</script><p>after script</p></body></html>');
        put(out, 'malformed_html', '<html><body><p>unclosed <b>bold<p>next</body>');
        }
        if (part == null || part == 'B') {        // null
        Map<String, Object> n = new Map<String, Object>();
        try { String s = null; Blob b = Blob.toPdf(s); n.put('size', b == null ? -1 : b.size()); }
        catch (Exception e) { n.put('exc', e.getTypeName() + '|' + e.getMessage()); }
        out.put('null', n);
        // 1 MB
        String big = '<html><body>' + 'x'.repeat(1000000) + '</body></html>';
        put(out, 'one_mb_text_run', big);
        String bigP = '<html><body>';
        String para = '<p>' + 'word '.repeat(190) + '</p>';
        Integer reps = 1000000 / para.length();
        for (Integer i = 0; i < reps; i++) bigP += para;
        bigP += '</body></html>';
        out.put('one_mb_paras_len', bigP.length());
        put(out, 'one_mb_paras', bigP);
        // determinism
        Blob d1 = Blob.toPdf('<p>same</p>');
        Blob d2 = Blob.toPdf('<p>same</p>');
        Map<String, Object> det = new Map<String, Object>();
        det.put('equal', d1 == d2);
        det.put('equal_hex', EncodingUtil.convertToHex(d1) == EncodingUtil.convertToHex(d2));
        det.put('size1', d1.size()); det.put('size2', d2.size());
        det.put('info1', info(d1)); det.put('info2', info(d2));
        out.put('determinism', det);
        }
        out.put('isTest', Test.isRunningTest());
        return JSON.serialize(out);
    }

    public static void put(Map<String, Object> out, String id, String input) {
        Map<String, Object> f = new Map<String, Object>();
        try {
            Blob b = Blob.toPdf(input);
            f.putAll(facets(b));
        } catch (Exception e) {
            f.put('exc', e.getTypeName() + '|' + e.getMessage());
        }
        out.put(id, f);
    }

    public static Boolean compact = false;
    public static Map<String, Object> facets(Blob b) {
        Map<String, Object> f = new Map<String, Object>();
        f.put('size', b.size());
        if (compact) {
            Map<String, Object> i = b.size() < 400000 ? info(b) : new Map<String, Object>();
            f.put('hx', EncodingUtil.convertToHex(b).left(32));
            f.put('p', i.get('producer')); f.put('fo', i.get('fonts')); f.put('mb', i.get('mediaBox')); f.put('pc', i.get('pageCount')); f.put('im', i.get('hasImage')); f.put('n', i.get('objCount'));
            return f;
        }
        String hex = EncodingUtil.convertToHex(b);
        f.put('head_hex', hex.left(32));
        f.put('b64_8', EncodingUtil.base64Encode(b).left(8));
        if (b.size() < 400000) f.put('info', info(b)); else f.put('info', 'skipped: size');
        return f;
    }

    // Extracts ASCII-visible metadata by searching the hex encoding (the PDF has binary stream bytes,
    // so Blob.toString() throws StringException: BLOB is not a valid UTF-8 string).
    public static Map<String, Object> info(Blob b) {
        return infoFromHex(EncodingUtil.convertToHex(b));
    }
    static String hx(String ascii) { return EncodingUtil.convertToHex(Blob.valueOf(ascii)); }
    static String unhx(String hex) {
        String r = '';
        for (Integer i = 0; i + 1 < hex.length() && i < 400; i += 2) {
            Integer v = hexVal(hex.substring(i, i + 2));
            r += (v >= 32 && v < 127) ? String.fromCharArray(new List<Integer>{ v }) : '?';
        }
        return r;
    }
    static Integer hexVal(String h) {
        String d = '0123456789abcdef';
        return d.indexOf(h.substring(0, 1)) * 16 + d.indexOf(h.substring(1, 2));
    }
    static Integer findEven(String hex, String needleHex, Integer start) {
        Integer i = hex.indexOf(needleHex, start);
        while (i >= 0 && Math.mod(i, 2) == 1) i = hex.indexOf(needleHex, i + 1);
        return i;
    }
    static String hexBetween(String hex, String a, String b) {
        String ah = hx(a), bh = hx(b);
        Integer i = findEven(hex, ah, 0);
        if (i < 0) return null;
        Integer j = findEven(hex, bh, i + ah.length());
        if (j < 0) return null;
        return unhx(hex.substring(i + ah.length(), j));
    }
    public static Map<String, Object> infoFromHex(String hex) {
        Map<String, Object> f = new Map<String, Object>();
        f.put('header', unhx(hex.left(30)));
        f.put('producer', hexBetween(hex, '/Producer(', ')'));
        f.put('creator', hexBetween(hex, '/Creator(', ')'));
        f.put('creationDate', hexBetween(hex, '/CreationDate(', ')'));
        f.put('modDate', hexBetween(hex, '/ModDate(', ')'));
        f.put('id', hexBetween(hex, '/ID [', ']'));
        f.put('mediaBox', hexBetween(hex, '/MediaBox[', ']'));
        f.put('mediaBox2', hexBetween(hex, '/MediaBox [', ']'));
        f.put('pageCount', hexBetween(hex, '/Type/Pages/Count ', '>'));
        f.put('version', hexBetween(hex, '%PDF-', '\n'));
        List<String> fonts = new List<String>();
        String bf = hx('/BaseFont/');
        Integer idx = 0;
        while (fonts.size() < 20) {
            idx = findEven(hex, bf, idx);
            if (idx < 0) break;
            Integer e1 = findEven(hex, hx('/'), idx + bf.length());
            Integer e2 = findEven(hex, hx('>'), idx + bf.length());
            Integer e = Math.min(e1 < 0 ? e2 : e1, e2 < 0 ? e1 : e2);
            fonts.add(unhx(hex.substring(idx + bf.length(), e)));
            idx = e;
        }
        f.put('fonts', fonts);
        f.put('hasImage', findEven(hex, hx('/Subtype/Image'), 0) >= 0);
        f.put('hasXObject', findEven(hex, hx('/XObject'), 0) >= 0);
        f.put('hasFlate', findEven(hex, hx('/FlateDecode'), 0) >= 0);
        f.put('hasFontFile', findEven(hex, hx('/FontFile'), 0) >= 0);
        f.put('hasToUnicode', findEven(hex, hx('/ToUnicode'), 0) >= 0);
        f.put('hasObjStm', findEven(hex, hx('/ObjStm'), 0) >= 0);
        f.put('objCount', hex.countMatches(hx(' 0 obj')));
        return f;
    }

    // Full base64 of one small render, for offline parsing.
    public static String sampleB64(String html) {
        return EncodingUtil.base64Encode(Blob.toPdf(html));
    }

    public static String runPages() {
        Map<String, Object> out = new Map<String, Object>();
        PageReference p = Page.PdfProbePage;
        try { out.put('getContentAsPDF', facetsSafe(p.getContentAsPDF())); } catch (Exception e) { out.put('getContentAsPDF', e.getTypeName() + '|' + e.getMessage()); }
        try { Blob c = p.getContent(); Map<String, Object> f = new Map<String,Object>(); f.put('size', c.size()); f.put('head', c.toString().left(40)); out.put('getContent_pdfpage', f); } catch (Exception e) { out.put('getContent_pdfpage', e.getTypeName() + '|' + e.getMessage()); }
        PageReference h = Page.PdfProbeHtmlPage;
        try { Blob c = h.getContent(); Map<String, Object> f = new Map<String,Object>(); f.put('size', c.size()); f.put('head', c.toString().left(60)); out.put('getContent_htmlpage', f); } catch (Exception e) { out.put('getContent_htmlpage', e.getTypeName() + '|' + e.getMessage()); }
        try { out.put('getContentAsPDF_htmlpage', facetsSafe(h.getContentAsPDF())); } catch (Exception e) { out.put('getContentAsPDF_htmlpage', e.getTypeName() + '|' + e.getMessage()); }
        PageReference p2 = Page.PdfProbePage;
        try { Blob a = p.getContentAsPDF(); Blob b2 = p2.getContentAsPDF(); out.put('page_determinism', a == b2); } catch (Exception e) { out.put('page_determinism', e.getTypeName() + '|' + e.getMessage()); }
        out.put('isTest', Test.isRunningTest());
        return JSON.serialize(out);
    }
    static Object facetsSafe(Blob b) {
        Map<String, Object> f = new Map<String, Object>();
        f.put('size', b.size());
        f.put('head_hex', EncodingUtil.convertToHex(b).left(32));
        try { f.put('info', info(b)); } catch (Exception e) { f.put('info_exc', e.getTypeName() + '|' + e.getMessage()); }
        return f;
    }
}
