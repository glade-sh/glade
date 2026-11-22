RasterBuffer buffer = new RasterBuffer();
List<Integer> sentinel = new List<Integer>();
for (Integer i = 0; i < 4096; i++) {
    buffer.pixels.add(Math.mod(i, 256));
}
Integer sum = 0;
for (Integer pixel : buffer.pixels) {
    sum += pixel;
}
System.assertEquals(4096, buffer.pixels.size());
System.assertEquals(522240, sum);
