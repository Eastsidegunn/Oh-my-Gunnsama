package hook

import (
	"context"
	"sort"
)

type Chain struct {
	hooks []Hook
}

func NewChain(hooks []Hook) Chain {
	sorted := make([]Hook, len(hooks))
	copy(sorted, hooks)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority() < sorted[j].Priority()
	})
	return Chain{hooks: sorted}
}

// hardSlots are slots where a hook error or deny aborts the operation.
var hardSlots = map[Slot]bool{
	SlotInput: true,
	SlotGuard: true,
}

func (c Chain) Apply(ctx context.Context, slot Slot, in *Input) (Output, error) {
	agg := Output{Allow: true}
	for _, h := range c.hooks {
		if h.Slot() != slot {
			continue
		}
		out, err := h.Apply(ctx, in)
		if err != nil {
			if hardSlots[slot] {
				return agg, err
			}
			agg.Warnings = append(agg.Warnings, "hook "+h.ID()+" error: "+err.Error())
			continue
		}
		agg.Warnings = append(agg.Warnings, out.Warnings...)
		if hardSlots[slot] && !out.Allow {
			agg.Allow = false
			agg.Reason = out.Reason
			return agg, nil
		}
		if out.FlowDecision != nil {
			agg.FlowDecision = out.FlowDecision
		}
	}
	return agg, nil
}

func (c Chain) Len() int { return len(c.hooks) }

func (c Chain) Hooks() []Hook {
	out := make([]Hook, len(c.hooks))
	copy(out, c.hooks)
	return out
}
