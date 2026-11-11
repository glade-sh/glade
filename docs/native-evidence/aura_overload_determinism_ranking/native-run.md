Capture date 2026-11-10. Three fresh 30-day Developer scratch orgs on campaign Dev Hubs, each deleted after capture (the delete command returned success for every org).

Question: if Salesforce is deterministic for String versus Integer, which equal-arity single-parameter overload does it choose for other parameter type pairs, and do arguments, declaration order or API matter?

Synthetic project: for every unordered pair of twelve parameter types (Boolean, Integer, Long, Decimal, Double, String, Id, Date, Datetime, Object, List<String>, Account) a method `pNxM` declares the two overloads. Class TA54 declares the lower-numbered type first, TB54 declares the higher-numbered type first (both Apex API 54), TA45 repeats TA54 at API 45. Each overload returns the index of its own type. Lightning web components at API 59 and 67 called every method through a Visualforce-hosted Lightning Out application with a missing parameter and with null. Class TA54 was also called with the string 'x' and with the number 42 in orgs t2 and t3.

Arms: t1 and t2 bulk deploy of all classes; t3 sequential per-class deploys in reverse order.

Result: 2,904 observations over 66 pairs. Every pair selected the same overload in every observation. The chosen type was the same for missing and null, both declaration orders, both Apex APIs, both component APIs and all three orgs. Wins per type (11 of 11 down to 0): List<String> 11, Datetime 10, Decimal 9, String 8, Object 7, Long 6, Integer 5, Double 4, Boolean 3, Account 2, Date 1, Id 0, so the preference is a strict total order. Argument values do not change the choice: with 'x' or 42 the winner is still invoked, and when the winner cannot convert the value the call fails with the parameter error instead of falling back to the other overload.
