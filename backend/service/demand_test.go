package service

import "testing"

func TestDemandTrackerRecordsAllCategories(t *testing.T) {
	d := NewDemandTracker()
	d.RecordRequest([]string{"horizontal", "vertical", "r18", "__uncategorized__"}, true)

	snap := d.Snapshot()
	for _, tag := range []string{"horizontal", "vertical", "r18"} {
		if snap[tag] != 1 {
			t.Errorf("tag %q not recorded (got %d)", tag, snap[tag])
		}
	}
	if _, ok := snap["_any"]; ok {
		t.Errorf("_any should not be recorded for categorized requests")
	}
}

func TestDemandTrackerEmptyCategoriesNoRecord(t *testing.T) {
	d := NewDemandTracker()
	d.RecordRequest(nil, true)
	d.RecordRequest([]string{"", "__uncategorized__"}, false)
	if len(d.Snapshot()) != 0 {
		t.Errorf("empty/uncategorized requests should not create demand, got %v", d.Snapshot())
	}
}

func TestGetAllocationPlanProportional(t *testing.T) {
	d := NewDemandTracker()
	d.RecordRequest([]string{"horizontal"}, true)
	d.RecordRequest([]string{"horizontal"}, true)
	d.RecordRequest([]string{"vertical"}, true)
	d.RecordRequest([]string{"r18"}, false) // miss -> guarantee

	plan := d.GetAllocationPlan(20, map[string]int{})
	if plan["r18"] < 1 {
		t.Errorf("missed tag r18 should get a guarantee slot, got %d", plan["r18"])
	}
	total := 0
	for _, n := range plan {
		total += n
	}
	if total > 20 {
		t.Errorf("allocation plan exceeded pool size: %d", total)
	}
	if plan["horizontal"] <= plan["vertical"] {
		t.Errorf("higher-demand tag horizontal should get >= vertical: %d vs %d", plan["horizontal"], plan["vertical"])
	}
}

func TestGetAllocationPlanGuaranteeCapped(t *testing.T) {
	d := NewDemandTracker()
	// 大量 miss 标签: 保底应按缺货数量收缩, 总量不超池
	for i := 0; i < 8; i++ {
		d.RecordRequest([]string{string(rune('a' + i))}, false)
	}
	plan := d.GetAllocationPlan(10, map[string]int{})
	total := 0
	for _, n := range plan {
		total += n
	}
	if total > 10 {
		t.Errorf("guarantee cap overshot pool: total=%d", total)
	}
	for tag, n := range plan {
		if n < 1 {
			t.Errorf("tag %q got 0 slots", tag)
		}
	}
}
