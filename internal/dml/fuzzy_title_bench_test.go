package dml

import "testing"

// BenchmarkFuzzyTitleScan scores one candidate title against 5,000 stored
// titles of two to four words, one of which matches: the shape of a custom
// Title rule's duplicate scan.
func BenchmarkFuzzyTitleScan(b *testing.B) {
	heads := []string{"Senior", "Regional", "Assistant", "Lead", "Chief", "Deputy", "Global", "Interim", "Associate", "Principal"}
	areas := []string{"Sales", "Marketing", "Finance", "Operations", "Support", "Logistics", "Research", "Quality", "Security", "Training"}
	roles := []string{"Manager", "Director", "Analyst", "Engineer", "Officer", "Planner", "Auditor", "Advisor", "Specialist", "Coordinator"}
	stored := make([]string, 0, 5000)
	for i := 0; len(stored) < 5000; i++ {
		title := areas[i%10] + " " + roles[(i/10)%10]
		switch i % 4 {
		case 1:
			title = heads[(i/100)%10] + " " + title
		case 2:
			title = roles[(i/10)%10] + " of " + areas[i%10]
		case 3:
			title = heads[(i/100)%10] + " " + title + " " + heads[(i/7)%10]
		}
		stored = append(stored, title)
	}
	stored[4321] = "Vice President of Customer Success"
	const candidate = "VP Customer Success"
	b.ReportAllocs()
	for b.Loop() {
		matches := 0
		for _, title := range stored {
			if ok, _ := fuzzyTitle(candidate, title, false); ok {
				matches++
			}
		}
		if matches != 1 {
			b.Fatalf("matches %d", matches)
		}
	}
}
