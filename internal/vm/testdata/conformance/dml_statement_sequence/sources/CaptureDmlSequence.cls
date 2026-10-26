public class CaptureDmlSequence {
    public static List<List<Integer>> observe() {
        List<List<Integer>> rows=new List<List<Integer>>();
        Integer emptyBefore=Limits.getDmlStatements(); Integer emptyLimit=Limits.getLimitDmlStatements(); insert new List<Account>();
        rows.add(new List<Integer>{-1,emptyBefore,emptyLimit,Limits.getDmlStatements(),Limits.getLimitDmlStatements(),Limits.getCpuTime(),Limits.getCpuTime()});
        for(Integer i=0;i<150;i++) {
            Integer before=Limits.getDmlStatements(); Integer limitBefore=Limits.getLimitDmlStatements(); Integer cpuBefore=Limits.getCpuTime();
            insert new Account(Name='Capture '+i);
            rows.add(new List<Integer>{i,before,limitBefore,Limits.getDmlStatements(),Limits.getLimitDmlStatements(),cpuBefore,Limits.getCpuTime()});
        }
        return rows;
    }
    public static void overflow() { for(Integer i=0;i<151;i++) insert new Account(Name='Overflow '+i); }
}
