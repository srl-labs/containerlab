package velocpu

import (
	"testing"
)

func setEnv(t *testing.T, pool, oversub string) {
	t.Helper()
	if pool != "" {
		t.Setenv(EnvPool, pool)
	}
	if oversub != "" {
		t.Setenv(EnvOversubscribe, oversub)
	}
	Reset()
	t.Cleanup(Reset)
}

func claim(t *testing.T, cores int) Allocation {
	t.Helper()
	a, err := Claim(cores)
	if err != nil {
		t.Fatalf("Claim(%d): %v", cores, err)
	}
	return a
}

func TestClaimDedicatedDisjoint(t *testing.T) {
	setEnv(t, "0-7", "1")

	if a := claim(t, 2); a.CPUSet != "0-1" || a.CPUQuota != 2 {
		t.Fatalf("first = %+v, want {0-1 2}", a)
	}
	if a := claim(t, 4); a.CPUSet != "2-5" || a.CPUQuota != 4 {
		t.Fatalf("second = %+v, want {2-5 4}", a)
	}
	if a := claim(t, 2); a.CPUSet != "6-7" || a.CPUQuota != 2 {
		t.Fatalf("third = %+v, want {6-7 2}", a)
	}
}

// 4 CPUs, factor 2, four 2-CPU nodes: two share 0-1, two share 2-3, each capped
// at 1.0 CPU. Mirrors the intended uniform over-provisioning.
func TestOversubscribeUniform(t *testing.T) {
	setEnv(t, "0-3", "2")

	want := []Allocation{
		{CPUSet: "0-1", CPUQuota: 1}, // new block 0-1
		{CPUSet: "2-3", CPUQuota: 1}, // new block 2-3 (pool now exhausted)
		{CPUSet: "0-1", CPUQuota: 1}, // stack onto least-occupied 0-1
		{CPUSet: "2-3", CPUQuota: 1}, // stack onto 2-3
	}
	for i, w := range want {
		if got := claim(t, 2); got != w {
			t.Fatalf("claim %d = %+v, want %+v", i+1, got, w)
		}
	}

	// fifth node exceeds factor on every block -> error
	if _, err := Claim(2); err == nil {
		t.Fatal("expected oversubscription-full error, got nil")
	}
}

func TestOversubscribeQuotaFraction(t *testing.T) {
	setEnv(t, "0-1", "4")
	// 2-CPU block shared by up to 4 nodes -> 0.5 CPU each.
	if a := claim(t, 2); a.CPUSet != "0-1" || a.CPUQuota != 0.5 {
		t.Fatalf("got %+v, want {0-1 0.5}", a)
	}
}

func TestReserveThenClaimSkipsReserved(t *testing.T) {
	setEnv(t, "0-7", "")

	if err := Reserve("2-3"); err != nil {
		t.Fatalf("Reserve(2-3): %v", err)
	}
	if a := claim(t, 4); a.CPUSet != "0-1,4-5" {
		t.Fatalf("claim after reserve = %+v, want cpu-set 0-1,4-5", a)
	}
}

func TestReserveOverlapErrors(t *testing.T) {
	setEnv(t, "0-7", "")
	claim(t, 2) // takes 0-1
	if err := Reserve("1-2"); err == nil {
		t.Fatal("expected overlap error, got nil")
	}
}

func TestReserveOutsidePoolErrors(t *testing.T) {
	setEnv(t, "0-3", "")
	if err := Reserve("4-5"); err == nil {
		t.Fatal("expected out-of-pool error, got nil")
	}
}

func TestBadOversubscribeErrors(t *testing.T) {
	setEnv(t, "0-3", "0")
	if _, err := Claim(2); err == nil {
		t.Fatal("expected error for oversubscribe < 1, got nil")
	}
}

func TestFormatCPUSet(t *testing.T) {
	cases := map[string]struct {
		in   []int
		want string
	}{
		"single":     {[]int{5}, "5"},
		"range":      {[]int{2, 3, 4}, "2-4"},
		"split":      {[]int{2, 3, 4, 7}, "2-4,7"},
		"gaps":       {[]int{0, 2, 4}, "0,2,4"},
		"two ranges": {[]int{0, 1, 4, 5}, "0-1,4-5"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := formatCPUSet(tc.in); got != tc.want {
				t.Fatalf("formatCPUSet(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseCPUSet(t *testing.T) {
	got, err := parseCPUSet("0-2,5,7-8")
	if err != nil {
		t.Fatalf("parseCPUSet: %v", err)
	}
	want := []int{0, 1, 2, 5, 7, 8}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	if _, err := parseCPUSet("3-1"); err == nil {
		t.Fatal("expected error for reversed range")
	}
}
