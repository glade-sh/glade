trigger ContactRewrite on Contact (before update) {
    for (Contact c : Trigger.new) {
        if (c.Description != null && c.Description.startsWith('rw|')) {
            List<String> p = c.Description.split('\\|');
            c.put(p[1], p[2]);
        }
    }
}
