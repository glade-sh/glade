public class ScopeBad { public static void run() { Map<String,Object> best = new Map<String,Object>(); if (true) { Integer line = 5; best = line; } } }
