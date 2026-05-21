package hook

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func newOrderHook(id string, priority int, slot Slot, order *[]string) Hook {
	return NewFunc(id, slot, priority, func(_ context.Context, _ *Input) (Output, error) {
		*order = append(*order, id)
		return Output{Allow: true}, nil
	})
}

func TestChain_AppliesInPriorityOrder(t *testing.T) {
	var order []string
	hooks := []Hook{
		newOrderHook("30", 30, SlotInput, &order),
		newOrderHook("10", 10, SlotInput, &order),
		newOrderHook("20", 20, SlotInput, &order),
	}
	chain := NewChain(hooks)

	out, err := chain.Apply(context.Background(), SlotInput, &Input{Slot: SlotInput})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Allow {
		t.Fatalf("expected Allow=true, got false (reason=%q)", out.Reason)
	}
	want := []string{"10", "20", "30"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("priority order mismatch: got %v want %v", order, want)
	}
}

func TestChain_FilterBySlot(t *testing.T) {
	var inputOrder, outputOrder []string
	hooks := []Hook{
		newOrderHook("in-a", 10, SlotInput, &inputOrder),
		newOrderHook("out-a", 10, SlotOutput, &outputOrder),
		newOrderHook("in-b", 20, SlotInput, &inputOrder),
		newOrderHook("out-b", 20, SlotOutput, &outputOrder),
	}
	chain := NewChain(hooks)

	if _, err := chain.Apply(context.Background(), SlotInput, &Input{Slot: SlotInput}); err != nil {
		t.Fatalf("input apply: %v", err)
	}
	if !reflect.DeepEqual(inputOrder, []string{"in-a", "in-b"}) {
		t.Fatalf("input slot fired wrong hooks: %v", inputOrder)
	}
	if len(outputOrder) != 0 {
		t.Fatalf("output hooks should not fire on SlotInput, got %v", outputOrder)
	}

	if _, err := chain.Apply(context.Background(), SlotOutput, &Input{Slot: SlotOutput}); err != nil {
		t.Fatalf("output apply: %v", err)
	}
	if !reflect.DeepEqual(outputOrder, []string{"out-a", "out-b"}) {
		t.Fatalf("output slot fired wrong hooks: %v", outputOrder)
	}
	if !reflect.DeepEqual(inputOrder, []string{"in-a", "in-b"}) {
		t.Fatalf("input order mutated after SlotOutput call: %v", inputOrder)
	}
}

func TestChain_InputBailShortCircuits(t *testing.T) {
	counter := 0
	deny := NewFunc("deny", SlotInput, 10, func(_ context.Context, _ *Input) (Output, error) {
		return Output{Allow: false, Reason: "nope"}, nil
	})
	counterHook := NewFunc("counter", SlotInput, 20, func(_ context.Context, _ *Input) (Output, error) {
		counter++
		return Output{Allow: true}, nil
	})
	chain := NewChain([]Hook{deny, counterHook})

	out, err := chain.Apply(context.Background(), SlotInput, &Input{Slot: SlotInput})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Allow {
		t.Fatalf("expected Allow=false after deny hook")
	}
	if out.Reason != "nope" {
		t.Fatalf("expected reason 'nope', got %q", out.Reason)
	}
	if counter != 0 {
		t.Fatalf("counter hook must not run after short-circuit, got counter=%d", counter)
	}
}

func TestChain_OutputCollectsAllWarnings(t *testing.T) {
	warn := func(id string, priority int, msg string) Hook {
		return NewFunc(id, SlotOutput, priority, func(_ context.Context, _ *Input) (Output, error) {
			return Output{Warnings: []string{msg}}, nil
		})
	}
	hooks := []Hook{
		warn("c", 30, "w3"),
		warn("a", 10, "w1"),
		warn("b", 20, "w2"),
	}
	chain := NewChain(hooks)

	out, err := chain.Apply(context.Background(), SlotOutput, &Input{Slot: SlotOutput})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"w1", "w2", "w3"}
	if !reflect.DeepEqual(out.Warnings, want) {
		t.Fatalf("warnings mismatch: got %v want %v", out.Warnings, want)
	}
}

func TestChain_InputErrorIsHard(t *testing.T) {
	sentinel := errors.New("boom")
	bad := NewFunc("bad", SlotInput, 10, func(_ context.Context, _ *Input) (Output, error) {
		return Output{}, sentinel
	})
	called := false
	after := NewFunc("after", SlotInput, 20, func(_ context.Context, _ *Input) (Output, error) {
		called = true
		return Output{Allow: true}, nil
	})
	chain := NewChain([]Hook{bad, after})

	_, err := chain.Apply(context.Background(), SlotInput, &Input{Slot: SlotInput})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error to bubble up, got %v", err)
	}
	if called {
		t.Fatalf("hooks after a hard error must not run")
	}
}

func TestChain_OutputErrorIsSoft(t *testing.T) {
	bad := NewFunc("bad-out", SlotOutput, 10, func(_ context.Context, _ *Input) (Output, error) {
		return Output{}, errors.New("kaboom")
	})
	after := NewFunc("after-out", SlotOutput, 20, func(_ context.Context, _ *Input) (Output, error) {
		return Output{Warnings: []string{"still ran"}}, nil
	})
	chain := NewChain([]Hook{bad, after})

	out, err := chain.Apply(context.Background(), SlotOutput, &Input{Slot: SlotOutput})
	if err != nil {
		t.Fatalf("output errors must be soft, got hard error: %v", err)
	}
	if len(out.Warnings) != 2 {
		t.Fatalf("expected 2 warnings (1 from err, 1 from after), got %d: %v", len(out.Warnings), out.Warnings)
	}
	if !strings.Contains(out.Warnings[0], "hook bad-out error: kaboom") {
		t.Fatalf("first warning should describe the error, got %q", out.Warnings[0])
	}
	if out.Warnings[1] != "still ran" {
		t.Fatalf("second warning should be from 'after-out', got %q", out.Warnings[1])
	}
}

func TestHookFunc_Adapter(t *testing.T) {
	invoked := false
	h := NewFunc("test-id", SlotInput, 42, func(_ context.Context, _ *Input) (Output, error) {
		invoked = true
		return Output{Allow: true}, nil
	})
	if h.ID() != "test-id" {
		t.Fatalf("ID(): got %q want %q", h.ID(), "test-id")
	}
	if h.Slot() != SlotInput {
		t.Fatalf("Slot(): got %v want %v", h.Slot(), SlotInput)
	}
	if h.Priority() != 42 {
		t.Fatalf("Priority(): got %d want 42", h.Priority())
	}
	if _, err := h.Apply(context.Background(), &Input{}); err != nil {
		t.Fatalf("Apply unexpected error: %v", err)
	}
	if !invoked {
		t.Fatalf("Apply did not invoke wrapped closure")
	}
}

func TestChain_EmptyChain(t *testing.T) {
	chain := NewChain(nil)
	out, err := chain.Apply(context.Background(), SlotInput, &Input{Slot: SlotInput})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Allow {
		t.Fatalf("empty chain must allow by default")
	}
	if len(out.Warnings) != 0 {
		t.Fatalf("empty chain should produce no warnings, got %v", out.Warnings)
	}
}

func TestChain_Len(t *testing.T) {
	hooks := []Hook{
		NewFunc("a", SlotInput, 10, nil),
		NewFunc("b", SlotInput, 20, nil),
		NewFunc("c", SlotOutput, 30, nil),
	}
	chain := NewChain(hooks)
	if got := chain.Len(); got != 3 {
		t.Fatalf("Len(): got %d want 3", got)
	}
}

func TestChain_GuardSlotBlocksTool(t *testing.T) {
	h := NewFunc("deny-bash", SlotGuard, 10, func(_ context.Context, in *Input) (Output, error) {
		if in.ToolCall != nil && in.ToolCall.Name == "bash" {
			return Output{Allow: false, Reason: "bash denied"}, nil
		}
		return Output{Allow: true}, nil
	})
	chain := NewChain([]Hook{h})

	out, err := chain.Apply(context.Background(), SlotGuard, &Input{
		Slot:     SlotGuard,
		ToolCall: &ToolCall{Name: "bash", Phase: "before"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Allow {
		t.Errorf("bash should be blocked by guard hook")
	}
	if out.Reason != "bash denied" {
		t.Errorf("Reason=%q, want 'bash denied'", out.Reason)
	}
}

func TestChain_GuardSlotAllowsOtherTools(t *testing.T) {
	h := NewFunc("deny-bash", SlotGuard, 10, func(_ context.Context, in *Input) (Output, error) {
		if in.ToolCall != nil && in.ToolCall.Name == "bash" {
			return Output{Allow: false, Reason: "bash denied"}, nil
		}
		return Output{Allow: true}, nil
	})
	chain := NewChain([]Hook{h})

	out, err := chain.Apply(context.Background(), SlotGuard, &Input{
		Slot:     SlotGuard,
		ToolCall: &ToolCall{Name: "read", Phase: "before"},
	})
	if err != nil || !out.Allow {
		t.Errorf("read should be allowed, err=%v allow=%v", err, out.Allow)
	}
}

func TestChain_GuardErrorIsHard(t *testing.T) {
	h := NewFunc("err-guard", SlotGuard, 10, func(_ context.Context, _ *Input) (Output, error) {
		return Output{}, errors.New("guard exploded")
	})
	chain := NewChain([]Hook{h})
	_, err := chain.Apply(context.Background(), SlotGuard, &Input{Slot: SlotGuard})
	if err == nil || !strings.Contains(err.Error(), "guard exploded") {
		t.Errorf("guard error should propagate hard, got %v", err)
	}
}

func TestChain_LifecycleSlotSoftError(t *testing.T) {
	h := NewFunc("lifecycle-err", SlotLifecycle, 10, func(_ context.Context, _ *Input) (Output, error) {
		return Output{}, errors.New("lifecycle boom")
	})
	chain := NewChain([]Hook{h})
	out, err := chain.Apply(context.Background(), SlotLifecycle, &Input{Slot: SlotLifecycle})
	if err != nil {
		t.Errorf("lifecycle error should be soft, got hard err: %v", err)
	}
	if len(out.Warnings) == 0 || !strings.Contains(out.Warnings[0], "lifecycle boom") {
		t.Errorf("lifecycle error should appear as warning, got %v", out.Warnings)
	}
}

func TestChain_FlowSlotPropagatesDecision(t *testing.T) {
	h := NewFunc("stop-flow", SlotFlow, 10, func(_ context.Context, _ *Input) (Output, error) {
		return Output{Allow: true, FlowDecision: &FlowDecision{Continue: false, Reason: "done"}}, nil
	})
	chain := NewChain([]Hook{h})
	out, err := chain.Apply(context.Background(), SlotFlow, &Input{Slot: SlotFlow})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.FlowDecision == nil {
		t.Fatal("FlowDecision should be set")
	}
	if out.FlowDecision.Continue {
		t.Errorf("Continue=true, want false")
	}
	if out.FlowDecision.Reason != "done" {
		t.Errorf("Reason=%q, want 'done'", out.FlowDecision.Reason)
	}
}

func TestChain_AllFiveSlots_Independent(t *testing.T) {
	slots := []Slot{SlotInput, SlotOutput, SlotGuard, SlotLifecycle, SlotFlow}
	for _, slot := range slots {
		var fired []string
		h := NewFunc("h-"+string(slot), slot, 10, func(_ context.Context, _ *Input) (Output, error) {
			fired = append(fired, string(slot))
			return Output{Allow: true}, nil
		})
		chain := NewChain([]Hook{h})
		for _, other := range slots {
			fired = nil
			chain.Apply(context.Background(), other, &Input{Slot: other})
			if other == slot && len(fired) != 1 {
				t.Errorf("slot %q: hook should fire when slot matches, fired=%v", slot, fired)
			}
			if other != slot && len(fired) != 0 {
				t.Errorf("slot %q: hook should NOT fire for slot %q, fired=%v", slot, other, fired)
			}
		}
	}
}
