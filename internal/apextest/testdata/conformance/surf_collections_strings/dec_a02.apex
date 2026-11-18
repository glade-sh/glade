List<String> out = new List<String>();
Set<Decimal> s = new Set<Decimal>{ 1.5 };
out.add('A02\tSet\t1.5\tset_membership\t\t\t\t' + String.valueOf(s.hashCode()) + '\t\t' + s.size() + '\t'
    + String.valueOf(s.contains(1.50)) + '\tadd_1.5_contains_1.50');
out.add('A02\tSet\t1.5\tset_membership\t\t\t\t\t\t' + s.size() + '\t'
    + String.valueOf(s.contains(Decimal.valueOf('1.500'))) + '\tcontains_1.500');
Set<Decimal> z = new Set<Decimal>{ Decimal.valueOf('0') };
out.add('A02\tSet\t0\tset_membership\t\t\t\t' + String.valueOf(z.hashCode()) + '\t\t' + z.size() + '\t'
    + String.valueOf(z.contains(0.00)) + '\tadd_0_contains_0.00');
Set<Decimal> n = new Set<Decimal>{ Decimal.valueOf('-0.01') };
out.add('A02\tSet\t-0.01\tset_membership\t\t\t\t' + String.valueOf(n.hashCode()) + '\t\t' + n.size() + '\t'
    + String.valueOf(n.contains(-0.0100)) + '\tadd_-0.01_contains_-0.0100');
out.add('END');
System.debug(String.join(out, '
'));
