public with sharing class FlowCoreAOAddDays {
    public static List<Integer> sizes = new List<Integer>();
    @InvocableMethod(label='Synthetic output boundary')
    public static List<Datetime> addDays(List<Request> requests) {
        sizes.add(requests.size());
        List<Datetime> results = new List<Datetime>();
        for (Request request : requests) {
            if (request.mode == 'throw') throw new BoundaryException('synthetic rejection');
            if (request.mode == 'null-list') return null;
            if (request.mode == 'empty') continue;
            if (request.mode == 'null-element') { results.add(null); continue; }
            results.add(request.startDateTime.addDays(requests.size()));
            if (request.mode == 'multiple') results.add(request.startDateTime.addDays(20));
        }
        return results;
    }
    public class BoundaryException extends Exception {}
    public class Request {
        @InvocableVariable public Datetime startDateTime;
        @InvocableVariable public String mode;
    }
}
