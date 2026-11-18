@IsTest private class LazyGetterTest62 {
 static List<Integer> source = new List<Integer>{1, 2};
 class Full { List<Integer> entries { get { if (this.entries == null) { this.entries = source; } return this.readBacking(); } private set; } List<Integer> readBacking() { return this.entries; } }
 class GuardOnly { List<Integer> entries { get { if (this.entries == null) { } return this.readBacking(); } private set; } List<Integer> readBacking() { return this.entries; } }
 class AssignOnly { List<Integer> entries { get { this.entries = source; return this.readBacking(); } private set; } List<Integer> readBacking() { return this.entries; } }
 class Plain { List<Integer> entries { get { return this.readBacking(); } private set; } List<Integer> readBacking() { return this.entries; } }
 @IsTest static void full() { System.assertEquals(2, new Full().entries.size()); }
 @IsTest static void guardOnly() { Object o = new GuardOnly().entries; }
 @IsTest static void assignOnly() { System.assertEquals(2, new AssignOnly().entries.size()); }
 @IsTest static void plain() { Object o = new Plain().entries; }
}
