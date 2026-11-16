package dml

import (
	"encoding/json"
	"os"
	"testing"
)

// BenchmarkCityZipPanels scores every ordered pair of the captured City and
// ZIP panels under the custom methods and the standard item scorers.
func BenchmarkCityZipPanels(b *testing.B) {
	var panels []nativePanel
	data, err := os.ReadFile("testdata/phone_city_zip/panels.json")
	if err != nil {
		b.Fatal(err)
	}
	if err := json.Unmarshal(data, &panels); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, panel := range panels {
			if panel.Method == "Phone" {
				continue
			}
			for _, x := range panel.Values {
				for _, y := range panel.Values {
					FuzzyScore(panel.Method, x, y, false)
					if panel.Method == "City" {
						standardCity(x, y)
					} else {
						standardZip(x, y)
					}
				}
			}
		}
	}
}
