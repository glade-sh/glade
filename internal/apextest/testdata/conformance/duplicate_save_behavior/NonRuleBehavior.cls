public class NonRuleBehavior {
    public static String ordinary() {
        return StandardBehavior.find(new Opportunity(Name='Example',StageName='Prospecting',CloseDate=Date.today()));
    }
}
