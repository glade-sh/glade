public class ExperienceTypeUse {
    public static String observe() {
        return String.valueOf([SELECT Id FROM User WHERE UserType = 'CustomerSuccess'].size());
    }
}
