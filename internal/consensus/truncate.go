package consensus

import "sort"

// TruncateResponses returns a copy of responses where, if the total character
// count exceeds maxTotalChars, the longest responses are proportionally
// truncated so the combined length fits within the budget. Truncated entries
// have "[truncated]" appended.
func TruncateResponses(responses map[string]string, maxTotalChars int) map[string]string {
	if maxTotalChars <= 0 {
		return responses
	}

	// Compute total length.
	total := 0
	for _, v := range responses {
		total += len(v)
	}

	// Nothing to do if we are within budget.
	if total <= maxTotalChars {
		out := make(map[string]string, len(responses))
		for k, v := range responses {
			out[k] = v
		}
		return out
	}

	const suffix = "[truncated]"
	suffixLen := len(suffix)

	// Build a sortable slice so behaviour is deterministic.
	type entry struct {
		id  string
		len int
	}
	entries := make([]entry, 0, len(responses))
	for id, v := range responses {
		entries = append(entries, entry{id: id, len: len(v)})
	}
	// Sort shortest first so we can apply the proportional budget in a
	// single pass from smallest to largest.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].len != entries[j].len {
			return entries[i].len < entries[j].len
		}
		return entries[i].id < entries[j].id
	})

	// Distribute the budget proportionally. Walk from the shortest entry
	// to the longest. For each entry compute its fair share of the
	// remaining budget. If the entry already fits, leave it alone and
	// subtract its length from the remaining budget. Otherwise cap it.
	remaining := maxTotalChars
	n := len(entries)
	caps := make(map[string]int, n)

	for i, e := range entries {
		fair := remaining / (n - i)
		if e.len <= fair {
			// Fits within its share -- keep it as-is.
			caps[e.id] = e.len
			remaining -= e.len
		} else {
			// Needs truncation -- cap at its fair share.
			cap := fair
			// Ensure there is room for the suffix.
			if cap < suffixLen {
				cap = suffixLen
			}
			caps[e.id] = cap
			remaining -= cap
			if remaining < 0 {
				remaining = 0
			}
		}
	}

	out := make(map[string]string, len(responses))
	for id, v := range responses {
		cap := caps[id]
		if len(v) <= cap {
			out[id] = v
		} else {
			// Reserve room for the suffix.
			cutoff := cap - suffixLen
			if cutoff < 0 {
				cutoff = 0
			}
			out[id] = v[:cutoff] + suffix
		}
	}

	return out
}
