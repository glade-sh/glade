public class StopOrderMx {
    public class BoomException extends Exception {}
    public static void enq(String name) {
        try {
            System.enqueueJob(new StopOrderQ(name, false));
        } catch (Exception e) {
            StopOrderMk.mark(name + '!' + e.getTypeName());
        }
    }
    public static void chain(String p, Integer total, Boolean capped) {
        try {
            if (capped) {
                AsyncOptions opts = new AsyncOptions();
                opts.MaximumQueueableStackDepth = 5;
                System.enqueueJob(new StopOrderChain(p, 1, total), opts);
            } else {
                System.enqueueJob(new StopOrderChain(p, 1, total));
            }
        } catch (Exception e) {
            StopOrderMk.mark(p + '0!' + e.getTypeName());
        }
    }
    @future public static void futChain(String name, String p, Integer total, Boolean capped) {
        StopOrderMk.mark(name);
        chain(p, total, capped);
    }
    @future public static void futFut(String name, String next) {
        StopOrderMk.mark(name);
        try {
            StopOrderMk.fut(next);
        } catch (Exception e) {
            StopOrderMk.mark(name + '!' + e.getTypeName());
        }
    }
}
