package pack

// An OverlapParser looks for overlapping matches and chooses the best ones,
// using an algorithm based on
// https://fastcompression.blogspot.com/2011/12/advanced-parsing-strategies.html
type OverlapParser struct {
	matchCache1, matchCache2 []AbsoluteMatch
}

func (p *OverlapParser) Parse(dst []Match, src Searcher, start, end int) []Match {
	s := start
	nextEmit := start
	matchesHere := p.matchCache1[:0]
	matchList := p.matchCache2[:0]

	for s < end {
		matchList = matchList[:0]
		matchesHere = src.Search(matchesHere[:0], s, nextEmit, end)
		m := longestMatch(matchesHere)
		if m.End-m.Start < 4 {
			s++
			continue
		}
		matchList = append(matchList, m)

		for {
			// Look for a new match overlapping the end of m.
			matchesHere = src.Search(matchesHere[:0], m.End-2, m.Start, end)
			newMatch := longestMatch(matchesHere)
			if newMatch.End-newMatch.Start <= m.End-m.Start {
				// It's no longer than the previous match, so ignore it.
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
					matchList[i+1].Match += matchList[i].End - matchList[i+1].Start
					matchList[i+1].Start = matchList[i].End
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
					matchList[i].End = matchList[i+1].Start
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
	p.matchCache1 = matchesHere[:0]
	p.matchCache2 = matchList[:0]
	return dst
}
