Map<String, Object> rows = new Map<String, Object>();
Map<String, List<Integer>> dates = new Map<String, List<Integer>>{
    'year_one_start' => new List<Integer>{1, 1, 1},
    'year_one_next' => new List<Integer>{1, 1, 2},
    'year_one_third' => new List<Integer>{1, 1, 3},
    'year_one_end' => new List<Integer>{1, 12, 31},
    'julian_leap' => new List<Integer>{1500, 2, 29},
    'last_julian' => new List<Integer>{1582, 10, 4},
    'first_gap' => new List<Integer>{1582, 10, 5},
    'gap' => new List<Integer>{1582, 10, 10},
    'last_gap' => new List<Integer>{1582, 10, 14},
    'first_gregorian' => new List<Integer>{1582, 10, 15},
    'gregorian_control' => new List<Integer>{1699, 12, 31}
};
for (String key : dates.keySet()) {
    try {
        List<Integer> parts = dates.get(key);
        Datetime dt = Datetime.newInstanceGmt(parts[0], parts[1], parts[2], 12, 34, 56);
        String encoded = JSON.serialize(dt);
        rows.put(key + '_construct', new List<Object>{dt.formatGmt('yyyy-MM-dd HH:mm:ss'), dt.getTime(), encoded});
        try {
            Datetime decoded = (Datetime)JSON.deserialize(encoded, Datetime.class);
            rows.put(key + '_round_trip', new List<Object>{
                decoded.getTime(), decoded == dt, JSON.serialize(decoded)
            });
        } catch (Exception e) {
            rows.put(key + '_round_trip', new List<String>{e.getTypeName(), e.getMessage()});
        }
    } catch (Exception e) {
        rows.put(key + '_construct', new List<String>{e.getTypeName(), e.getMessage()});
    }
}
for (String input : new List<String>{
    '0000-01-01T00:00:00.000Z', '0000-02-29T12:34:56.000Z',
    '0000-12-30T12:34:56.123+02:00', '0000-02-30T12:34:56.000Z',
    '0000-12-30T12:34:56.000Z', '0000-12-31T12:34:56.000Z',
    '0001-01-01T12:34:56.000Z', '1582-10-10T12:34:56.123Z', '1699-12-31T12:34:56.123Z',
    '1582-02-30T12:34:56.000Z', '0001-01-01T12:34:56.000+02:00'
}) {
    for (Boolean strict : new List<Boolean>{false, true}) {
        String key = input + (strict ? '_strict' : '_lax');
        try {
            Datetime decoded = (Datetime)(strict ? JSON.deserializeStrict(JSON.serialize(input), Datetime.class)
                : JSON.deserialize(JSON.serialize(input), Datetime.class));
            rows.put(key, new List<Object>{decoded.getTime(), JSON.serialize(decoded)});
        } catch (Exception e) {
            rows.put(key, new List<String>{e.getTypeName()});
        }
    }
}
