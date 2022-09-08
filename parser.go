package pack

// An AbsoluteMatch is like a Match, but it stores indexes into the byte
// stream instead of lengths.
type AbsoluteMatch struct {
	// Start is the index of the first byte.
	Start int

	// End is the index of the byte after the last byte
	// (so that End - Start = Length).
	End int

	// Match is the index of the previous data that matches
	// (Start - Match = Distance).
	Match int
}

// A Searcher is the source of matches for a Parser. It is a lower-level
// interface than MatchFinder, only looking for matches at one position at a
// time. A type that uses a Parser to implement MatchFinder can implement
// Searcher as well, and pass itself to the Parser.
type Searcher interface {
	// Search looks for matches at pos and appends them to dst.
	// In each match, Start and End must fall within the interval [min,max),
	// and Match < Start < End.
	Search(dst []AbsoluteMatch, pos, min, max int) []AbsoluteMatch
}

// A Parser chooses which matches to use to compress the data.
type Parser interface {
	// Parse gets matches from src, chooses which ones to use, and appends
	// them to dst. The matches cover the range of bytes from start to end.
	Parse(dst []Match, src Searcher, start, end int) []Match
}

// A GreedyParser implements the greedy matching strategy: It goes from start
// to end, choosing the longest match at each position.
type GreedyParser struct {
	matchCache []AbsoluteMatch
}

func (p *GreedyParser) Parse(dst []Match, src Searcher, start, end int) []Match {
	matches := p.matchCache[:0]
	s := start
	nextEmit := start
	var m AbsoluteMatch

mainLoop:
	for {
		nextS := s
		for {
			s = nextS
			nextS = s + 1
			if nextS >= end {
				break mainLoop
			}

			matches = src.Search(matches[:0], s, nextEmit, end)
			m = longestMatch(matches)
			if m.End >= m.Start+4 {
				break
			}
		}

		dst = append(dst, Match{
			Unmatched: m.Start - nextEmit,
			Length:    m.End - m.Start,
			Distance:  m.Start - m.Match,
		})
		s = m.End
		nextEmit = s
	}

	if nextEmit < end {
		dst = append(dst, Match{
			Unmatched: end - nextEmit,
		})
	}
	p.matchCache = matches[:0]
	return dst
}

func longestMatch(matches []AbsoluteMatch) AbsoluteMatch {
	var longest AbsoluteMatch

	for _, m := range matches {
		if m.End-m.Start > longest.End-longest.Start {
			longest = m
		}
	}

	return longest
}
