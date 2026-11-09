List<Integer> l = new List<Integer>(); for (Integer i = 1; i <= 11; i++) l.add(i); Set<Integer> s = new Set<Integer>(l); System.assertEquals(s, new Set<Integer>());
