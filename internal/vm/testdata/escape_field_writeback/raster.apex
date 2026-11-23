RasterBuffer buffer = new RasterBuffer();
List<Integer> sentinel = new List<Integer>();
for (Integer i = 0; i < 20000; i++) {
    buffer.pixels.add(Math.mod(i, 256));
}
Integer sum = 0;
for (Integer pixel : buffer.pixels) {
    sum += pixel;
}
System.assertEquals(20000, buffer.pixels.size());
System.assertEquals(2546416, sum);
