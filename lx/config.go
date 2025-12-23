package lx

// Options holds CLI-level configuration before effective values are derived.
type Options struct {
	Head  int
	Tail  int
	NBoth int

	HeadSet bool
	TailSet bool
	NSet    bool

	PrefixDelimiter  string
	PostfixDelimiter string
	LineNumbers      bool
	ShowTokens       bool
}

// Effective derives a fully configured Runner from the options, applying
// -n / --head / --tail override rules.
func (o Options) Effective() Runner {
	effHead := o.Head
	effTail := o.Tail

	if o.NSet && o.NBoth > 0 {
		total := o.NBoth

		switch {
		case !o.HeadSet && !o.TailSet:
			// Pure -n N: split N between head and tail, head gets extra on odd N.
			effHead = (total + 1) / 2
			effTail = total / 2

		case o.HeadSet && !o.TailSet:
			// -n N with explicit --head: keep head, derive tail as the remainder.
			h := o.Head
			if h < 0 {
				h = 0
			}
			if h > total {
				h = total
			}
			effHead = h
			effTail = total - h

		case !o.HeadSet && o.TailSet:
			// -n N with explicit --tail: keep tail, derive head as the remainder.
			t := o.Tail
			if t < 0 {
				t = 0
			}
			if t > total {
				t = total
			}
			effTail = t
			effHead = total - t

		case o.HeadSet && o.TailSet:
			// Both explicitly set; respect them and let -n only be a shorthand
			// for "I care about both ends" without overriding explicit values.
			// effHead / effTail already initialized from o.Head / o.Tail.
		}
	}

	return NewRunner(
		effHead,
		effTail,
		o.PrefixDelimiter,
		o.PostfixDelimiter,
		o.LineNumbers,
		o.ShowTokens,
	)
}
