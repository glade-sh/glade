package sema

import "sync"

// A serial body pass carries one member view through every type, so a type
// can see entries that earlier types hydrated. semaAnyKnownField is the only
// reader whose answer depends on that history (checkMethodBodiesSpeculative
// relies on the same fact). The incremental pass checks each type on a view
// that starts from the pre-body history and records, per type, which answers
// depended on history and which fields the type made visible. Validation then
// replays the serial order over those records.
//
// Two bounds drive it. A field in the self set of an earlier valid type is
// visible on the serial history: that type adds it whenever its own answers
// are unchanged. A field that no type has ever made visible, in this or any
// earlier generation of the state, is not visible: every hydrated field was
// first added by some type or by the passes before the bodies. A "known"
// answer needs the first bound; an "unknown" answer needs the second.

// semaHydrationTrace attributes one worker view's hydration to the type the
// worker is checking. Each worker owns its trace; pre and oracle are shared
// and read-only.
type semaHydrationTrace struct {
	// pre holds the hydrated field names visible before any body task ran.
	pre map[string]struct{}
	// oracle maps a field name to the first valid type known to make it
	// visible. A type after that one answers "known" for it.
	oracle map[string]int
	// seen holds the hydrated field names this worker's view has since shown.
	seen   map[string]struct{}
	cur    *semaTypeHydration
	worker int
	round  int
	// unsafe is raised when a stored entry dropped a field the replaced entry
	// had. Visibility then is not monotone, and no record can be trusted.
	unsafe bool
}

// semaTypeHydration is one type's body-pass hydration record.
type semaTypeHydration struct {
	typeIndex, worker, round int
	// yes holds names the view answered "known" only through an entry this
	// type did not add itself.
	yes map[string]struct{}
	// assumed holds names the oracle answered "known" for.
	assumed map[string]struct{}
	// no holds names answered "unknown".
	no map[string]struct{}
	// self holds field names this type made visible first in its view.
	self map[string]struct{}
}

func newSemaHydrationTrace(pre map[string]struct{}, oracle map[string]int, worker, round int) *semaHydrationTrace {
	return &semaHydrationTrace{pre: pre, oracle: oracle, seen: make(map[string]struct{}), worker: worker, round: round}
}

func (t *semaHydrationTrace) begin(typeIndex int) *semaTypeHydration {
	t.cur = &semaTypeHydration{typeIndex: typeIndex, worker: t.worker, round: t.round}
	return t.cur
}

func semaHydratedFieldSet(view *semaTypeMemberView) map[string]struct{} {
	out := make(map[string]struct{})
	for _, members := range view.hydrated {
		for name := range members.fields {
			out[name] = struct{}{}
		}
	}
	return out
}

func (t *semaHydrationTrace) store(previous typeMembers, replaced bool, members typeMembers) {
	if replaced {
		for name := range previous.fields {
			if _, ok := members.fields[name]; !ok {
				t.unsafe = true
			}
		}
	}
	for name := range members.fields {
		if _, ok := t.pre[name]; ok {
			continue
		}
		if _, ok := t.seen[name]; ok {
			continue
		}
		t.seen[name] = struct{}{}
		if t.cur != nil {
			if t.cur.self == nil {
				t.cur.self = make(map[string]struct{})
			}
			t.cur.self[name] = struct{}{}
		}
	}
}

// answer records one semaAnyKnownField answer for the current type and
// returns the answer the serial pass gives, as far as the oracle knows it.
func (t *semaHydrationTrace) answer(model *semaTypeMemberView, key string, known bool) bool {
	rec := t.cur
	if rec == nil {
		return known
	}
	if !known {
		if first, ok := t.oracle[key]; ok && first < rec.typeIndex {
			semaAddName(&rec.assumed, key)
			return true
		}
		semaAddName(&rec.no, key)
		return false
	}
	if semaAnyKnownFieldInView(model, key, true) {
		return true
	}
	if _, own := rec.self[key]; !own {
		// A field the type added itself before asking is there on every
		// history; any other hydrated field came from an earlier type.
		semaAddName(&rec.yes, key)
	}
	return true
}

func semaAddName(set *map[string]struct{}, key string) {
	if *set == nil {
		*set = make(map[string]struct{})
	}
	(*set)[key] = struct{}{}
}

// resetBodyMemos clears the caches a view keeps across types, so each type's
// lookups, and therefore its record, are its own.
func (v *semaTypeMemberView) resetBodyMemos() {
	v.bodyFieldScopeMemo = nil
	v.methodFamilies = nil
	v.lastMethodFamilyValid = false
	v.lexicalShadows = nil
}

// semaHydrationCheck is the outcome of one validation walk.
type semaHydrationCheck struct {
	// rerun marks the types to check again.
	rerun []bool
	// reruns counts them.
	reruns int
	// firstSelf maps each field a valid type made visible to the first such
	// type; it is the next round's oracle.
	firstSelf map[string]int
	// conflict reports a rechecked "unknown" answer that an earlier type may
	// contradict but no earlier type settles; only a full analysis can.
	conflict bool
}

// semaValidateHydration walks the types in serial order. pre is the hydrated
// field set the serial body pass starts with; ever holds every field an
// earlier generation saw made visible. A reused type is valid when its
// "known" answers have a visible source and its "unknown" answers name no
// field that any type can make visible. A rechecked type ran on pre plus
// earlier types of its own worker, which the serial view also holds, so its
// "known" view answers stand; its assumed and "unknown" answers are checked
// like a reused type's. A rechecked type that is not valid may have hydrated
// what the serial pass does not, so every later type of the same worker and
// round runs again too.
func semaValidateHydration(records []*semaTypeHydration, rechecked []bool, pre, ever map[string]struct{}) semaHydrationCheck {
	check := semaHydrationCheck{rerun: make([]bool, len(records)), firstSelf: make(map[string]int)}
	// An "unknown" answer can be wrong only for a field some type adds.
	possible := func(name string) bool {
		if _, ok := pre[name]; ok {
			return true
		}
		if _, ok := ever[name]; ok {
			return true
		}
		for _, rec := range records {
			if rec != nil {
				if _, ok := rec.self[name]; ok {
					return true
				}
			}
		}
		return false
	}
	contested := map[string]bool{}
	for _, rec := range records {
		if rec == nil {
			continue
		}
		for name := range rec.no {
			if _, done := contested[name]; !done {
				contested[name] = possible(name)
			}
		}
	}
	visible := func(name string) bool {
		if _, ok := pre[name]; ok {
			return true
		}
		_, ok := check.firstSelf[name]
		return ok
	}
	type workerRound struct{ worker, round int }
	tainted := map[workerRound]bool{}
	for typeIndex, rec := range records {
		if rec == nil {
			continue
		}
		valid := !(rechecked[typeIndex] && tainted[workerRound{rec.worker, rec.round}])
		if valid && !rechecked[typeIndex] {
			for name := range rec.yes {
				if !visible(name) {
					valid = false
					break
				}
			}
		}
		if valid {
			for name := range rec.assumed {
				if !visible(name) {
					valid = false
					break
				}
			}
		}
		if valid {
			for name := range rec.no {
				if !contested[name] {
					continue
				}
				if rechecked[typeIndex] && !visible(name) {
					check.conflict = true
					return check
				}
				valid = false
				break
			}
		}
		if !valid {
			check.rerun[typeIndex] = true
			check.reruns++
			if rechecked[typeIndex] {
				tainted[workerRound{rec.worker, rec.round}] = true
			}
			continue
		}
		for name := range rec.self {
			if _, ok := check.firstSelf[name]; !ok {
				check.firstSelf[name] = typeIndex
			}
		}
	}
	return check
}

// semaEverVisible extends ever with pre and every record's self set.
func semaEverVisible(ever, pre map[string]struct{}, records []*semaTypeHydration) map[string]struct{} {
	out := make(map[string]struct{}, len(ever)+len(pre))
	for name := range ever {
		out[name] = struct{}{}
	}
	for name := range pre {
		out[name] = struct{}{}
	}
	for _, rec := range records {
		if rec != nil {
			for name := range rec.self {
				out[name] = struct{}{}
			}
		}
	}
	return out
}

// semaLaterHistory answers semaAnyKnownField in the passes after an
// incremental body pass. The view there holds only the pre-body history; a
// field some valid type made visible is "known", as on the serial history.
// Any other "unknown" answer is kept for finish, which accepts it only for a
// field no type has made visible.
type semaLaterHistory struct {
	visible map[string]int
	mu      sync.Mutex
	missing map[string]struct{}
}

func (h *semaLaterHistory) answer(key string) bool {
	if _, ok := h.visible[key]; ok {
		return true
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	semaAddName(&h.missing, key)
	return false
}

func (h *semaLaterHistory) missingNames() map[string]struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.missing
}
