package eval

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The machine reports nothing at all until it has been warned, and that silence is the
// healthy state rather than a failed reading.
func TestThermalSilenceIsNominal(t *testing.T) {
	s := parseThermal("Note: No thermal warning level has been recorded\n" +
		"Note: No performance warning level has been recorded\n")
	if !s.OK || s.SpeedLimit != 100 {
		t.Fatalf("got %+v, want an OK sample at 100", s)
	}
}

func TestThermalReadsACap(t *testing.T) {
	s := parseThermal("CPU_Scheduler_Limit \t= 100\nCPU_Available_CPUs \t= 12\nCPU_Speed_Limit \t= 62\n")
	if !s.OK || s.SpeedLimit != 62 {
		t.Fatalf("got %+v, want 62", s)
	}
}

// A run that started capped is as void as one capped partway through: the question is
// whether the timing can be trusted, not when the cap arrived.
func TestThrottledTakesTheWorstOfThePair(t *testing.T) {
	nominal := ThermalSample{SpeedLimit: 100, OK: true}
	capped := ThermalSample{SpeedLimit: 70, OK: true}

	if throttled, _ := Throttled(nominal, nominal); throttled {
		t.Fatal("an unthrottled run must not be void")
	}
	if throttled, limit := Throttled(nominal, capped); !throttled || limit != 70 {
		t.Fatalf("a cap arriving during the run must void it, got %v %d", throttled, limit)
	}
	if throttled, limit := Throttled(capped, nominal); !throttled || limit != 70 {
		t.Fatalf("a run that started capped must void too, got %v %d", throttled, limit)
	}
	if throttled, _ := Throttled(ThermalSample{}, nominal); throttled {
		t.Fatal("an unreadable sample is not evidence of throttling")
	}
}

// The hash is over what came back, so two builds that decode the same text agree and two
// that do not, do not. An error must never be hashed: two failing builds would agree.
func TestFidelityHashesRepliesAndRefusesErrors(t *testing.T) {
	reply := "one"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, reply)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, 5*time.Second)

	first, err := Fidelity(context.Background(), c, 64)
	if err != nil {
		t.Fatalf("fidelity: %v", err)
	}
	same, err := Fidelity(context.Background(), c, 64)
	if err != nil || same.Hash != first.Hash {
		t.Fatalf("identical replies must hash alike: %v %v", same.Hash, err)
	}
	reply = "two"
	differs, err := Fidelity(context.Background(), c, 64)
	if err != nil {
		t.Fatalf("fidelity: %v", err)
	}
	if differs.Hash == first.Hash {
		t.Fatal("different replies must not hash alike")
	}
	if len(first.Replies) != len(FidelityProbes()) {
		t.Fatalf("kept %d replies for %d probes", len(first.Replies), len(FidelityProbes()))
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"error":{"message":"boom"}}`)
	}))
	defer bad.Close()
	if _, err := Fidelity(context.Background(), NewClient(bad.URL, 5*time.Second), 64); err == nil {
		t.Fatal("a server error must not be hashed as fidelity")
	}
}
