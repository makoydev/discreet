// Package bench measures how well Discreet finds personal data, using
// synthetic text from a generator written independently of the detection
// rules: its own NRIC checksum and Luhn code, its own templates, and
// deliberately realistic variants the rules may not handle. Synthetic data
// still flatters rule-based detectors; EVALS.md says so.
package bench

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

// Span is one piece of real personal data in a sample, by byte offset.
type Span struct {
	Entity     string
	Start, End int
	Variant    string // which writing style produced it
}

// Sample is one generated text with its true personal data.
type Sample struct {
	Text  string
	Spans []Span
}

type builder struct {
	b     strings.Builder
	spans []Span
}

func (b *builder) text(s string) { b.b.WriteString(s) }

func (b *builder) value(entity, variant, s string) {
	start := b.b.Len()
	b.b.WriteString(s)
	b.spans = append(b.spans, Span{entity, start, b.b.Len(), variant})
}

// gen draws values and their written forms.
type gen struct{ r *rand.Rand }

func (g gen) pick(items ...string) string { return items[g.r.IntN(len(items))] }
func (g gen) digits(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(byte('0' + g.r.IntN(10)))
	}
	return b.String()
}

// nric builds a synthetic NRIC/FIN with a correct check letter, computed
// here independently of internal/detect.
func (g gen) nric() string {
	prefix := g.pick("S", "T", "F", "G", "M")
	d := g.digits(7)
	weights := []int{2, 7, 6, 5, 4, 3, 2}
	sum := map[string]int{"S": 0, "T": 4, "F": 0, "G": 4, "M": 3}[prefix]
	for i, w := range weights {
		sum += w * int(d[i]-'0')
	}
	table := map[string]string{"S": "JZIHGFEDCBA", "T": "JZIHGFEDCBA", "F": "XWUTRQPNMLK", "G": "XWUTRQPNMLK", "M": "XWUTRQPNJLK"}[prefix]
	return prefix + d + string(table[sum%11])
}

func (g gen) luhnComplete(partial string) string {
	sum := 0
	for i := len(partial) - 1; i >= 0; i-- {
		d := int(partial[i] - '0')
		if (len(partial)-i)%2 == 1 {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return partial + string(byte('0'+(10-sum%10)%10))
}

func (g gen) card() (string, int) {
	switch g.r.IntN(3) {
	case 0:
		return g.luhnComplete("4" + g.digits(14)), 16
	case 1:
		return g.luhnComplete(g.pick("51", "52", "53", "54", "55") + g.digits(13)), 16
	default:
		return g.luhnComplete(g.pick("34", "37") + g.digits(12)), 15
	}
}

func groups(s string, sizes []int, sep string) string {
	var parts []string
	for _, n := range sizes {
		parts = append(parts, s[:n])
		s = s[n:]
	}
	return strings.Join(parts, sep)
}

var firstNames = []string{"tan", "lim", "lee", "ng", "wong", "goh", "chua", "ong", "koh", "teo", "raj", "siti", "ahmad", "mei", "wei"}
var streets = []string{"Ang Mo Kio Avenue 3", "Tampines Street 81", "Jurong West Street 52", "Bedok North Road", "Yishun Ring Road", "Toa Payoh Lorong 1", "Clementi Avenue 2"}
var months = []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}

// One writer per entity: it writes a value in one of several styles,
// including ones a rule-based detector may miss.
type writer func(g gen, b *builder)

var writers = map[string]writer{
	"NRIC": func(g gen, b *builder) {
		n := g.nric()
		switch g.r.IntN(10) {
		case 0:
			b.value("NRIC", "lowercase", strings.ToLower(n))
		case 1:
			b.value("NRIC", "spaced", n[:1]+" "+n[1:8]+" "+n[8:])
		case 2:
			b.value("NRIC", "hyphenated", n[:1]+"-"+n[1:8]+"-"+n[8:])
		case 3:
			wrong := n[:8] + string(byte('A'+(int(n[8]-'A')+1+g.r.IntN(24))%26))
			b.value("NRIC", "mistyped check letter", wrong)
		default:
			b.value("NRIC", "standard", n)
		}
	},
	"PHONE": func(g gen, b *builder) {
		n := g.pick("9", "8", "6", "9", "8") + g.digits(7)
		forms := [][2]string{
			{"plain", n}, {"4-4 space", n[:4] + " " + n[4:]}, {"4-4 hyphen", n[:4] + "-" + n[4:]},
			{"+65 space", "+65 " + n[:4] + " " + n[4:]}, {"+65 joined", "+65" + n}, {"(65)", "(65) " + n[:4] + " " + n[4:]},
			{"2-2-2-2", n[:2] + " " + n[2:4] + " " + n[4:6] + " " + n[6:]}, {"3-5", n[:3] + " " + n[3:]},
		}
		f := forms[g.r.IntN(len(forms))]
		b.value("PHONE", f[0], f[1])
	},
	"EMAIL": func(g gen, b *builder) {
		user := g.pick(firstNames...) + g.pick(".", "_", "", ".") + g.pick(firstNames...) + g.pick("", "", g.digits(2))
		domain := g.pick("example.com", "example.org", "mail.example.net", "clinic.example.com", "example.test")
		switch g.r.IntN(10) {
		case 0:
			b.value("EMAIL", "obfuscated", user+" at "+strings.ReplaceAll(domain, ".", " dot "))
		case 1:
			b.value("EMAIL", "uppercase", strings.ToUpper(user+"@"+domain))
		case 2:
			b.value("EMAIL", "plus tag", user+"+claims@"+domain)
		default:
			b.value("EMAIL", "standard", user+"@"+domain)
		}
	},
	"CARD": func(g gen, b *builder) {
		c, n := g.card()
		sizes := []int{4, 4, 4, 4}
		if n == 15 {
			sizes = []int{4, 6, 5}
		}
		switch g.r.IntN(8) {
		case 0:
			b.value("CARD", "plain", c)
		case 1:
			b.value("CARD", "hyphens", groups(c, sizes, "-"))
		case 2:
			b.value("CARD", "double spaces", groups(c, sizes, "  "))
		default:
			b.value("CARD", "spaced", groups(c, sizes, " "))
		}
	},
	"POSTAL": func(g gen, b *builder) {
		sector := 1 + g.r.IntN(82)
		if sector == 74 {
			sector = 73
		}
		code := fmt.Sprintf("%02d%s", sector, g.digits(4))
		switch g.r.IntN(8) {
		case 0:
			b.text("S(")
			b.value("POSTAL", "S(…)", code)
			b.text(")")
		case 1:
			b.text("postal code ")
			b.value("POSTAL", "postal code", code)
		case 2:
			b.text("S'pore ")
			b.value("POSTAL", "S'pore abbreviation", code)
		case 3:
			b.text(g.pick(streets...) + ", ")
			b.value("POSTAL", "no clue", code)
		default:
			b.text("Singapore ")
			b.value("POSTAL", "after Singapore", code)
		}
	},
	"UNIT": func(g gen, b *builder) {
		u := fmt.Sprintf("%02d-%0*d", 1+g.r.IntN(30), 2+g.r.IntN(2), g.r.IntN(1000))
		switch g.r.IntN(6) {
		case 0:
			b.text("Unit ")
			b.value("UNIT", "Unit without #", u)
		case 1:
			b.value("UNIT", "# with space", "# "+u)
		default:
			b.value("UNIT", "#floor-unit", "#"+u)
		}
	},
	"DOB": func(g gen, b *builder) {
		y, m, d := 1945+g.r.IntN(60), 1+g.r.IntN(12), 1+g.r.IntN(28)
		forms := []struct{ clue, variant, value string }{
			{"DOB: ", "DOB dd/mm/yyyy", fmt.Sprintf("%02d/%02d/%d", d, m, y)},
			{"Date of birth ", "date of birth d Month yyyy", fmt.Sprintf("%d %s %d", d, months[m-1], y)},
			{"born on ", "born on Month d, yyyy", fmt.Sprintf("%s %d, %d", months[m-1], d, y)},
			{"\"dateOfBirth\": \"", "JSON ISO", fmt.Sprintf("%d-%02d-%02d", y, m, d)},
			{"born in Singapore on ", "words between clue and date", fmt.Sprintf("%d/%d/%d", d, m, y)},
			{"DOB ", "two-digit year", fmt.Sprintf("%02d/%02d/%02d", d, m, y%100)},
			{"Birthday: ", "birthday clue", fmt.Sprintf("%02d.%02d.%d", d, m, y)},
		}
		f := forms[g.r.IntN(len(forms))]
		b.text(f.clue)
		b.value("DOB", f.variant, f.value)
		if strings.HasPrefix(f.clue, "\"") {
			b.text("\"")
		}
	},
}

// Decoys look like personal data but aren't; they measure false alarms.
var decoys = []func(g gen) string{
	func(g gen) string { return "order #" + g.digits(8) },
	func(g gen) string { return "invoice INV-2026" + g.digits(4) + "-" + g.digits(4) },
	func(g gen) string { return "amount SGD " + g.digits(3) + "," + g.digits(3) + ".00" },
	func(g gen) string { return "OTP " + g.digits(6) },
	func(g gen) string { return "ticket " + g.digits(6) },
	func(g gen) string { return "timestamp 17" + g.digits(11) },
	func(g gen) string { return "tracking " + g.pick("SG", "RR") + g.digits(9) + "SG" },
	func(g gen) string { return "version 2." + g.digits(1) + "." + g.digits(2) },
	func(g gen) string { return "ref S$" + g.digits(7) },
	func(g gen) string { return "meeting on " + g.digits(1) + " " + months[g.r.IntN(12)] + " 2026" },
	func(g gen) string { return "colour #" + g.pick("ff", "0a", "c3") + g.digits(4) },
	func(g gen) string { return "batch " + g.pick("8", "9", "6") + g.digits(7) },
	func(g gen) string { return "account 0" + g.digits(2) + "-" + g.digits(6) + "-" + g.digits(1) },
	func(g gen) string { return "issue #" + g.digits(2) + "-#" + g.digits(2) },
}

var openers = []string{
	"Hi team, ", "Customer note: ", "Please follow up. ", "From the claims queue: ", "Clinic intake. ",
	"Re: your enquiry. ", "Summary of call: ", "", "URGENT ", "log: ", "Fwd: ", "As discussed, ",
}
var joiners = []string{" and ", ", ", ". Also ", "; ", " - ", ". ", " (", "\n"}
var labels = map[string][]string{
	"NRIC":   {"NRIC ", "IC no. ", "FIN: ", "member ", "", "id="},
	"PHONE":  {"call ", "mobile ", "tel: ", "contact ", "", "hp "},
	"EMAIL":  {"email ", "reply to ", "", "cc ", "from: "},
	"CARD":   {"card ", "charged to ", "PAN ", "", "cc no "},
	"POSTAL": {"address ", "deliver to Blk 12, ", "", "home: "},
	"UNIT":   {"Blk 123 ", "office ", "", "flat "},
	"DOB":    {"", "patient ", "applicant "},
}

var entityOrder = []string{"NRIC", "PHONE", "EMAIL", "CARD", "POSTAL", "UNIT", "DOB"}

// Generate returns n samples from a fixed seed: the same seed always gives
// the same samples.
func Generate(n int, seed uint64) []Sample {
	g := gen{rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
	out := make([]Sample, 0, n)
	for i := 0; i < n; i++ {
		var b builder
		b.text(g.pick(openers...))
		items := g.r.IntN(4) // 0-3 pieces of personal data
		decoyCount := g.r.IntN(3)
		order := g.r.Perm(items + decoyCount)
		for k, slot := range order {
			if k > 0 {
				b.text(g.pick(joiners...))
			}
			if slot < items {
				entity := entityOrder[g.r.IntN(len(entityOrder))]
				b.text(g.pick(labels[entity]...))
				writers[entity](g, &b)
			} else {
				b.text(decoys[g.r.IntN(len(decoys))](g))
			}
		}
		b.text(g.pick(".", "", " thanks", ". Regards"))
		out = append(out, Sample{Text: b.b.String(), Spans: b.spans})
	}
	return out
}
