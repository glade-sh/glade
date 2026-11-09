List<Integer> nums(Integer n) { List<Integer> r = new List<Integer>(); for (Integer i = 0; i < n; i++) { r.add(i); } return r; }
Map<String, Integer> idx(List<String> s) { Map<String, Integer> m = new Map<String, Integer>(); for (Integer i = 0; i < s.size(); i++) { m.put(s[i], i); } return m; }
System.debug('CAP|lm|' + nums(3) + idx(new List<String>{'a', 'b'}));
