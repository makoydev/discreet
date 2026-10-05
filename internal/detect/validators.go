package detect

import (
	"regexp"
	"strconv"
	"strings"
)

// Validators named in detectors.json, implemented from sg-pii-rules'
// VALIDATORS.md. A detector naming a validator missing here fails to load.
var validators = map[string]func(string) bool{
	"sg_nric_fin_checksum":         isValidNRIC,
	"sg_nric_fin_checksum_invalid": hasNRICShapeButInvalidChecksum,
	"payment_card_luhn_iin":        isPaymentCard,
	"sg_postal_sector":             isPostalCode,
	"calendar_date":                isPlausibleDate,
}

// ----- NRIC/FIN -----

var (
	nricShape   = regexp.MustCompile(`^([STFGM])(\d{7})([A-Z])$`)
	nricWeights = [7]int{2, 7, 6, 5, 4, 3, 2}
	nricOffset  = map[string]int{"S": 0, "T": 4, "F": 0, "G": 4, "M": 3}
	nricLetters = map[string]string{
		"S": "JZIHGFEDCBA", "T": "JZIHGFEDCBA",
		"F": "XWUTRQPNMLK", "G": "XWUTRQPNMLK",
		"M": "XWUTRQPNJLK",
	}
)

// nricCheckLetter returns the expected check letter for a prefix and seven digits.
func nricCheckLetter(prefix, digits string) byte {
	sum := nricOffset[prefix]
	for i, w := range nricWeights {
		sum += w * int(digits[i]-'0')
	}
	return nricLetters[prefix][sum%11]
}

func isValidNRIC(value string) bool {
	m := nricShape.FindStringSubmatch(strings.ToUpper(value))
	return m != nil && nricCheckLetter(m[1], m[2]) == m[3][0]
}

func hasNRICShapeButInvalidChecksum(value string) bool {
	return nricShape.MatchString(strings.ToUpper(value)) && !isValidNRIC(value)
}

// ----- Payment cards -----

func passesLuhn(digits string) bool {
	sum := 0
	for i := 0; i < len(digits); i++ {
		d := int(digits[len(digits)-1-i] - '0')
		if i%2 == 1 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return sum%10 == 0
}

type cardNetwork struct {
	prefixes [][2]string // inclusive ranges of leading digits
	lengths  []int
}

var cardNetworks = []cardNetwork{
	{[][2]string{{"4", "4"}}, []int{13, 16, 19}},                                                      // Visa
	{[][2]string{{"51", "55"}, {"2221", "2720"}}, []int{16}},                                          // Mastercard
	{[][2]string{{"34", "34"}, {"37", "37"}}, []int{15}},                                              // American Express
	{[][2]string{{"6011", "6011"}, {"644", "649"}, {"65", "65"}, {"622126", "622925"}}, span(16, 19)}, // Discover
	{[][2]string{{"3528", "3589"}}, span(16, 19)},                                                     // JCB
	{[][2]string{{"62", "62"}}, span(16, 19)},                                                         // UnionPay
	{[][2]string{{"30", "30"}, {"36", "36"}, {"38", "39"}}, span(14, 19)},                             // Diners Club
}

func span(from, to int) []int {
	var s []int
	for n := from; n <= to; n++ {
		s = append(s, n)
	}
	return s
}

func hasCardNetworkPrefix(digits string) bool {
	for _, n := range cardNetworks {
		if !containsInt(n.lengths, len(digits)) {
			continue
		}
		for _, r := range n.prefixes {
			lead := digits[:len(r[0])]
			if lead >= r[0] && lead <= r[1] {
				return true
			}
		}
	}
	return false
}

func containsInt(list []int, n int) bool {
	for _, v := range list {
		if v == n {
			return true
		}
	}
	return false
}

var cardDigits = regexp.MustCompile(`^\d{13,19}$`)

func isPaymentCard(value string) bool {
	digits := strings.NewReplacer(" ", "", "-", "").Replace(value)
	return cardDigits.MatchString(digits) && passesLuhn(digits) && hasCardNetworkPrefix(digits)
}

// ----- Postal codes -----

var sixDigits = regexp.MustCompile(`^\d{6}$`)

// isPostalCode accepts six digits whose sector (first two) is 01-82, except 74.
func isPostalCode(value string) bool {
	if !sixDigits.MatchString(value) {
		return false
	}
	sector, _ := strconv.Atoi(value[:2])
	return sector >= 1 && sector <= 82 && sector != 74
}

// ----- Dates -----

var (
	numericDate = regexp.MustCompile(`^(\d{1,2})[/.-](\d{1,2})[/.-](\d{4})$`)
	isoDate     = regexp.MustCompile(`^(\d{4})-(\d{1,2})-(\d{1,2})$`)
	dayMonth    = regexp.MustCompile(`(?i)^(\d{1,2}) ([a-z]+)\.?,? (\d{4})$`)
	monthDay    = regexp.MustCompile(`(?i)^([a-z]+)\.? (\d{1,2}),? (\d{4})$`)
	months      = "janfebmaraprmayjunjulaugsepoctnovdec"
)

func isCalendarDate(year, month, day int) bool {
	if month < 1 || month > 12 || day < 1 {
		return false
	}
	leap := (year%4 == 0 && year%100 != 0) || year%400 == 0
	days := [12]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	if leap {
		days[1] = 29
	}
	return day <= days[month-1]
}

// monthNumber reads a month name by its first three letters; 0 if unknown.
func monthNumber(name string) int {
	if len(name) < 3 {
		return 0
	}
	i := strings.Index(months, strings.ToLower(name[:3]))
	if i < 0 || i%3 != 0 {
		return 0
	}
	return i/3 + 1
}

// isPlausibleDate accepts a real calendar date in the supported forms,
// reading 12/03/1988 day-first or month-first. It never compares with today.
func isPlausibleDate(value string) bool {
	n := func(s string) int { v, _ := strconv.Atoi(s); return v }
	if m := numericDate.FindStringSubmatch(value); m != nil {
		a, b, y := n(m[1]), n(m[2]), n(m[3])
		return isCalendarDate(y, b, a) || isCalendarDate(y, a, b)
	}
	if m := isoDate.FindStringSubmatch(value); m != nil {
		return isCalendarDate(n(m[1]), n(m[2]), n(m[3]))
	}
	if m := dayMonth.FindStringSubmatch(value); m != nil {
		return isCalendarDate(n(m[3]), monthNumber(m[2]), n(m[1]))
	}
	if m := monthDay.FindStringSubmatch(value); m != nil {
		return isCalendarDate(n(m[3]), monthNumber(m[1]), n(m[2]))
	}
	return false
}
