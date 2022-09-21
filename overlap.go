package pack

// An OverlapParser looks for overlapping matches and chooses the best ones,
// using an algorithm based on
// https://fastcompression.blogspot.com/2011/12/advanced-parsing-strategies.html
type OverlapParser struct {
	// Score is used to choose the best match. If it is nil,
	// the length of the match is used as its score.
	Score func(AbsoluteMatch) int

	matchCache []AbsoluteMatch
	setCache   []matchSet
}

func length(m AbsoluteMatch) int {
	return m.End - m.Start
}

type matchSet struct {
	AbsoluteMatch
	options []AbsoluteMatch
}

func (ms *matchSet) choose(score func(AbsoluteMatch) int) {
	ms.AbsoluteMatch = AbsoluteMatch{}
	maxScore := 0

	for _, m := range ms.options {
		s := score(m)
		if s > maxScore {
			ms.AbsoluteMatch = m
			maxScore = s
		}
	}
}

// trim chooses the best match from ms.options, with the range limited to
// min..max.
func (ms *matchSet) trim(min, max int, score func(AbsoluteMatch) int) {
	ms.AbsoluteMatch = AbsoluteMatch{}
	maxScore := 0

	for _, m := range ms.options {
		if m.Start < min {
			m.Match += min - m.Start
			m.Start = min
		}
		if m.End > max {
			m.End = max
		}
		if m.End <= m.Start {
			continue
		}
		s := score(m)
		if s > maxScore {
			ms.AbsoluteMatch = m
			maxScore = s
		}
	}
}

func (p *OverlapParser) Parse(dst []Match, src Searcher, start, end int) []Match {
	s := start
	nextEmit := start
	matchList := p.setCache[:0]

	if p.Score == nil {
		p.Score = length
	}

	for s < end {
		matchList = matchList[:0]

		p.matchCache = src.Search(p.matchCache[:0], s, nextEmit, end)
		m := matchSet{options: p.matchCache}
		m.choose(p.Score)
		if m.End-m.Start < 4 {
			s++
			continue
		}
		matchList = append(matchList, m)

		for {
			// Look for a new match overlapping the end of m.
			cacheLen := len(p.matchCache)
			p.matchCache = src.Search(p.matchCache, m.End-2, m.Start, end)
			newMatch := matchSet{options: p.matchCache[cacheLen:]}
			newMatch.choose(p.Score)
			if p.Score(newMatch.AbsoluteMatch) <= p.Score(m.AbsoluteMatch) {
				// It's no better than the previous match, so ignore it.
				break
			}
			m = newMatch
			matchList = append(matchList, m)
		}

		// We now have a series of overlapping matches,
		// each one longer than the previous one.
		// Now we need to resolve the overlaps.
		for i := len(matchList) - 2; i >= 0; i-- {
			if matchList[i].End-matchList[i].Start > matchList[i+1].End-matchList[i+1].Start {
				// This match is actually longer than the following one, probably because
				// the following one has already been trimmed.
				// So we'll trim the following one to remove the overlap with this match.
				if matchList[i].End > matchList[i+1].Start {
					if i < len(matchList)-2 {
						matchList[i+1].trim(matchList[i].End, matchList[i+2].Start, p.Score)
					} else {
						matchList[i+1].trim(matchList[i].End, end, p.Score)
					}
				}
				if matchList[i+1].End-matchList[i+1].Start < 4 {
					// The following match is too short now, so we'll just drop it.
					matchList = append(matchList[:i+1], matchList[i+2:]...)
					if i < len(matchList)-1 {
						// Run through the loop with the same index again,
						// to catch overlaps between this match and its new neighbor.
						i++
					}
				}
			} else {
				// The following match is longer than this one, so we'll trim this one
				// to remove the overlap.
				if matchList[i].End > matchList[i+1].Start {
					matchList[i].trim(nextEmit, matchList[i+1].Start, p.Score)
				}
				if matchList[i].End-matchList[i].Start < 4 {
					// This match is too short now, so we'll just drop it.
					matchList = append(matchList[:i], matchList[i+1:]...)
				}
			}
		}

		for _, m := range matchList {
			dst = append(dst, Match{
				Unmatched: m.Start - nextEmit,
				Length:    m.End - m.Start,
				Distance:  m.Start - m.Match,
			})
			nextEmit = m.End
		}
		s = nextEmit
	}

	if nextEmit < end {
		dst = append(dst, Match{
			Unmatched: end - nextEmit,
		})
	}
	p.setCache = matchList[:0]
	return dst
}
