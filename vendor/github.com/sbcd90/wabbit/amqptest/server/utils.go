package server

import "strings"

// matchs r2 against r1 following the AMQP rules for topic routing keys
func topicMatch(r1, r2 string) bool {
	var match bool

	bparts := strings.Split(r1, ".")
	rparts := strings.Split(r2, ".")

	if len(rparts) > len(bparts) {
		return false
	}

outer:
	for i := 0; i < len(bparts); i++ {
		bp := bparts[i]
		rp := rparts[i]

		if len(bp) == 0 {
			return false
		}

		var bsi, rsi int

		for rsi < len(rp) {
			// fmt.Printf("Testing '%c' and '%c'\n", bp[bsi], rp[rsi])

			// The char '#' matchs none or more chars (everything that is on rp[rsi])
			// next char, move on
			if bp[bsi] == '#' {
				match = true
				continue outer
			} else if bp[bsi] == '*' {
				// The '*' matchs only one character, then if it's the last char of binding part
				// and isn't the last char of rp, then surely it don't match.
				if bsi == len(bp)-1 && rsi < len(rp)-1 {
					match = false
					break outer
				}

				match = true

				if bsi < len(bp)-1 {
					bsi++
				}

				rsi++
			} else if bp[bsi] == rp[rsi] {
				// if it's the last char of binding part and it isn't an '*' or '#',
				// and it isn't the last char of rp, then we can stop here
				// because sure that route don't match the binding
				if bsi == len(bp)-1 && rsi < len(rp)-1 {
					match = false
					break outer
				}

				if bsi < len(bp)-1 {
					bsi++
				}

				rsi++

				match = true
			} else {
				match = false
				break outer
			}
		}

	}

	return match
}
