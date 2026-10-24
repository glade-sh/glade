public class PersonQueryStaticWhere {
    public static String observe() {
        return String.valueOf([SELECT Id FROM Account WHERE Loyalty__pc = null AND Id = null].size());
    }
}
