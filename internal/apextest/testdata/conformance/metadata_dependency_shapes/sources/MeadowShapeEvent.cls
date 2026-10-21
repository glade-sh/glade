public class MeadowShapeEvent {
    public static String observe() {
        Database.SaveResult result = EventBus.publish(new MeadowPulse__e(Note__c='sprout'));
        return String.valueOf(result.isSuccess());
    }
}
