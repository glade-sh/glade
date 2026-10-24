public class HeapLimitProbe {
    public static List<String> heldStrings;
    public static Map<String, String> heldValues;
    public static List<Blob> heldBlobs;
    public static Integer startHeap;
    public static Integer lastHeap;
    public static Integer heap10;
    public static Integer heap20;
    public static Integer heap30;
    public static Integer heap40;
    public static Integer heap50;
    public static Integer count;
    public static Integer payloadSize;
    public static Boolean complete;

    public static void grow(String kind, Integer target) {
        startHeap = Limits.getHeapSize();
        lastHeap = startHeap;
        heap10 = null;
        heap20 = null;
        heap30 = null;
        heap40 = null;
        heap50 = null;
        count = 0;
        payloadSize = null;
        complete = false;

        String chunk = 'x'.repeat(100000);
        heldStrings = new List<String>();
        heldValues = new Map<String, String>();
        heldBlobs = new List<Blob>();
        Integer used = Limits.getHeapSize();
        Integer payloadBytes = 0;
        while (count < 700 && (kind == 'blob' ? payloadBytes : used) < target) {
            String item = chunk + count;
            if (kind == 'list') {
                heldStrings.add(item);
            } else if (kind == 'map') {
                heldValues.put(String.valueOf(count), item);
            } else if (kind == 'blob') {
                heldBlobs.add(Blob.valueOf(item));
            }
            count++;
            payloadBytes = count * 100000;
            used = Limits.getHeapSize();
            lastHeap = used;
            if (heap10 == null && (used >= 10000000 || kind == 'blob' && payloadBytes >= 10000000)) heap10 = used;
            if (heap20 == null && (used >= 20000000 || kind == 'blob' && payloadBytes >= 20000000)) heap20 = used;
            if (heap30 == null && (used >= 30000000 || kind == 'blob' && payloadBytes >= 30000000)) heap30 = used;
            if (heap40 == null && (used >= 40000000 || kind == 'blob' && payloadBytes >= 40000000)) heap40 = used;
            if (heap50 == null && (used >= 50000000 || kind == 'blob' && payloadBytes >= 50000000)) heap50 = used;
        }
        if (kind == 'list' && !heldStrings.isEmpty()) {
            count = heldStrings.size();
            payloadSize = heldStrings[0].length();
        } else if (kind == 'map' && !heldValues.isEmpty()) {
            count = heldValues.size();
            payloadSize = heldValues.get('0').length();
        } else if (kind == 'blob' && !heldBlobs.isEmpty()) {
            count = heldBlobs.size();
            payloadSize = heldBlobs[0].size();
        }
        complete = (kind == 'blob' ? payloadBytes : used) >= target;
    }

    public static String marker(String kind, String route, String stage) {
        return 'P|HEAP5|kind=' + kind + '|route=' + route + '|stage=' + stage
            + '|count=' + count + '|start=' + startHeap + '|heap10=' + heap10
            + '|heap20=' + heap20 + '|heap30=' + heap30 + '|heap40=' + heap40
            + '|heap50=' + heap50 + '|last=' + lastHeap + '|payload=' + payloadSize
            + '|complete=' + complete;
    }
}
