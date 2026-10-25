package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// --- Alfred Script Filter JSON Schema ---

type AlfredMod struct {
	Subtitle string `json:"subtitle"`
	Arg      string `json:"arg"`
}

type AlfredItem struct {
	Title    string               `json:"title"`
	Subtitle string               `json:"subtitle"`
	Arg      string               `json:"arg"`
	Valid    bool                 `json:"valid"`
	Mods     map[string]AlfredMod `json:"mods,omitempty"`
}

type AlfredResponse struct {
	Items []AlfredItem `json:"items"`
}

func main() {
	if len(os.Args) < 2 || strings.TrimSpace(os.Args[1]) == "" {
		emitPrompt(
			"Type a Raycast expression...",
			"e.g., 32/10 USD in INR + 100  |  3h 40m + 2h 20m  |  24px + 1.5rem  |  5pm ldn in ist",
		)
		return
	}

	rawQuery := strings.TrimSpace(os.Args[1])
	q := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(rawQuery, "="), "="))
	if q == "" {
		emitPrompt("Type a calculation...", "Waiting for input...")
		return
	}

	type evaluator func(string) (string, string, string, bool)
	pipeline := []evaluator{
		evalTimezone,
		evalDates,
		evalUnifiedMath,
	}

	for _, fn := range pipeline {
		if formatted, raw, desc, ok := fn(q); ok {
			emitResult(q, formatted, raw, desc)
			return
		}
	}

	emitPrompt(
		"Unrecognized expression",
		"Use 'conv' for physical units or try: 32/10 USD in INR + 100 | 3h 40m + 2h 20m | 20% off 80",
	)
}

func emitResult(query, formatted, raw, desc string) {
	eq := fmt.Sprintf("%s = %s", query, formatted)
	resp := AlfredResponse{
		Items: []AlfredItem{
			{
				Title:    formatted,
				Subtitle: fmt.Sprintf("%s  •  ↩ Copy '%s'  |  ⌘↩ Raw '%s'  |  ⌥↩ Equation", desc, formatted, raw),
				Arg:      formatted,
				Valid:    true,
				Mods: map[string]AlfredMod{
					"cmd": {
						Subtitle: fmt.Sprintf("Copy unformatted answer: %s", raw),
						Arg:      raw,
					},
					"alt": {
						Subtitle: fmt.Sprintf("Copy question & answer: %s", eq),
						Arg:      eq,
					},
				},
			},
		},
	}
	out, _ := json.Marshal(resp)
	fmt.Println(string(out))
}

func emitPrompt(title, subtitle string) {
	resp := AlfredResponse{
		Items: []AlfredItem{{Title: title, Subtitle: subtitle, Valid: false}},
	}
	out, _ := json.Marshal(resp)
	fmt.Println(string(out))
}

// --- 1. Time Zone Engine ---

var tzAliases = map[string]string{
	"utc": "UTC", "gmt": "Europe/London", "ldn": "Europe/London", "london": "Europe/London", "uk": "Europe/London",
	"ist": "Asia/Kolkata", "india": "Asia/Kolkata", "hyd": "Asia/Kolkata",
	"est": "America/New_York", "edt": "America/New_York", "nyc": "America/New_York", "ny": "America/New_York",
	"pst": "America/Los_Angeles", "pdt": "America/Los_Angeles", "sf": "America/Los_Angeles", "la": "America/Los_Angeles",
	"cst": "America/Chicago", "chicago": "America/Chicago",
	"cet": "Europe/Berlin", "berlin": "Europe/Berlin", "paris": "Europe/Paris",
	"jst": "Asia/Tokyo", "tokyo": "Asia/Tokyo",
	"sgt": "Asia/Singapore", "singapore": "Asia/Singapore",
	"dubai": "Asia/Dubai", "sydney": "Australia/Sydney", "aest": "Australia/Sydney",
}

var (
	reTZNow     = regexp.MustCompile(`(?i)^(?:time|now)\s+in\s+([a-zA-Z/_]+)$`)
	reTZConvert = regexp.MustCompile(`(?i)^(?:(\d{1,2}):(\d{2})\s*(am|pm)?|(\d{1,2})\s*(am|pm))\s+([a-zA-Z/_]+)\s+(?:in|to|as)\s+([a-zA-Z/_]+)$`)
)

func resolveTZ(name string) (*time.Location, bool) {
	if iana, ok := tzAliases[strings.ToLower(name)]; ok {
		loc, err := time.LoadLocation(iana)
		return loc, err == nil
	}
	loc, err := time.LoadLocation(name)
	return loc, err == nil
}

func evalTimezone(q string) (string, string, string, bool) {
	if m := reTZNow.FindStringSubmatch(q); m != nil {
		loc, ok := resolveTZ(m[1])
		if !ok {
			return "", "", "", false
		}
		now := time.Now().In(loc)
		return now.Format("3:04 PM MST (Mon, 02 Jan)"), now.Format("15:04"), fmt.Sprintf("Current time in %s", loc.String()), true
	}

	if m := reTZConvert.FindStringSubmatch(q); m != nil {
		var hr, min int
		var ampm string
		if m[1] != "" {
			hr, _ = strconv.Atoi(m[1])
			min, _ = strconv.Atoi(m[2])
			ampm = strings.ToLower(m[3])
		} else {
			hr, _ = strconv.Atoi(m[4])
			ampm = strings.ToLower(m[5])
		}

		if ampm != "" {
			if hr < 1 || hr > 12 {
				return "", "", "", false
			}
			if ampm == "pm" && hr != 12 {
				hr += 12
			} else if ampm == "am" && hr == 12 {
				hr = 0
			}
		} else if hr > 23 || min > 59 {
			return "", "", "", false
		}

		fromLoc, ok1 := resolveTZ(m[6])
		toLoc, ok2 := resolveTZ(m[7])
		if !ok1 || !ok2 {
			return "", "", "", false
		}

		nowFrom := time.Now().In(fromLoc)
		src := time.Date(nowFrom.Year(), nowFrom.Month(), nowFrom.Day(), hr, min, 0, 0, fromLoc)
		dst := src.In(toLoc)

		srcDay := time.Date(src.Year(), src.Month(), src.Day(), 0, 0, 0, 0, time.UTC)
		dstDay := time.Date(dst.Year(), dst.Month(), dst.Day(), 0, 0, 0, 0, time.UTC)
		dayShift := int(dstDay.Sub(srcDay).Hours() / 24)

		suffix := ""
		if dayShift == 1 {
			suffix = " (+1 day)"
		} else if dayShift == -1 {
			suffix = " (-1 day)"
		}

		formatted := dst.Format("3:04 PM MST") + suffix
		return formatted, dst.Format("15:04"), fmt.Sprintf("%s → %s", src.Format("3:04 PM MST"), toLoc.String()), true
	}
	return "", "", "", false
}

// --- 2. Date Math Engine ---

var (
	reDateOffset  = regexp.MustCompile(`(?i)^(?:today|now)\s*([+-])\s*(\d+)\s*(days?|weeks?)$`)
	reWeekdayIn   = regexp.MustCompile(`(?i)^(sunday|monday|tuesday|wednesday|thursday|friday|saturday)\s+in\s+(\d+)\s*weeks?$`)
	reDaysUntil   = regexp.MustCompile(`(?i)^days\s+until\s+(.+)$`)
	weekdayLookup = map[string]time.Weekday{
		"sunday": time.Sunday, "monday": time.Monday, "tuesday": time.Tuesday,
		"wednesday": time.Wednesday, "thursday": time.Thursday, "friday": time.Friday, "saturday": time.Saturday,
	}
)

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

func evalDates(q string) (string, string, string, bool) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if m := reDateOffset.FindStringSubmatch(q); m != nil {
		n, _ := strconv.Atoi(m[2])
		if strings.HasPrefix(strings.ToLower(m[3]), "week") {
			n *= 7
		}
		if m[1] == "-" {
			n = -n
		}
		target := today.AddDate(0, 0, n)
		return target.Format("Mon, 02 Jan 2006"), target.Format("2006-01-02"), "Relative date calculation", true
	}

	if m := reWeekdayIn.FindStringSubmatch(q); m != nil {
		wd := weekdayLookup[strings.ToLower(m[1])]
		weeks, _ := strconv.Atoi(m[2])
		diff := int(wd) - int(today.Weekday())
		target := today.AddDate(0, 0, diff+(weeks*7))
		return target.Format("Mon, 02 Jan 2006"), target.Format("2006-01-02"), fmt.Sprintf("%s in %d week(s)", capitalize(m[1]), weeks), true
	}

	if m := reDaysUntil.FindStringSubmatch(q); m != nil {
		rawDate := strings.TrimSpace(m[1])
		layoutsWithYear := []string{"2 Jan 2006", "02 Jan 2006", "2 January 2006", "Jan 2 2006", "January 2 2006", "2006-01-02"}
		for _, l := range layoutsWithYear {
			if t, err := time.ParseInLocation(l, rawDate, now.Location()); err == nil {
				days := int(math.Round(t.Sub(today).Hours() / 24))
				return fmt.Sprintf("%d days", days), strconv.Itoa(days), fmt.Sprintf("Until %s", t.Format("Mon, 02 Jan 2006")), true
			}
		}
		layoutsNoYear := []string{"2 Jan", "2 January", "Jan 2", "January 2"}
		for _, l := range layoutsNoYear {
			if t, err := time.ParseInLocation(l, rawDate, now.Location()); err == nil {
				target := time.Date(today.Year(), t.Month(), t.Day(), 0, 0, 0, 0, now.Location())
				if target.Before(today) {
					target = target.AddDate(1, 0, 0)
				}
				days := int(math.Round(target.Sub(today).Hours() / 24))
				return fmt.Sprintf("%d days", days), strconv.Itoa(days), fmt.Sprintf("Until %s", target.Format("Mon, 02 Jan 2006")), true
			}
		}
	}
	return "", "", "", false
}

// --- 3. Unified Raycast Expression Engine (Math + Inline Currency + Time + CSS) ---

type unitKind int

const (
	kindScalar unitKind = iota
	kindCurrency
	kindTime
	kindCSS
)

type quantity struct {
	val       float64
	kind      unitKind
	unit      string
	explicit  bool
	isPercent bool
}

var (
	reRatioPct = regexp.MustCompile(`(?i)^(.+?)\s+(?:as\s+(?:a\s+)?%\s+of|is\s+what\s+%\s+of)\s+(.+)$`)
	reSqrtNL   = regexp.MustCompile(`(?i)\bsquare\s+root\s+of\b`)
	reBareCSS  = regexp.MustCompile(`(?i)^(-?\d+(?:\.\d+)?)\s*(px|rem)$`)
)

func evalUnifiedMath(q string) (string, string, string, bool) {
	baseRem := 16.0
	if envRem := os.Getenv("BASE_REM_PX"); envRem != "" {
		if parsed, err := strconv.ParseFloat(envRem, 64); err == nil && parsed > 0 {
			baseRem = parsed
		}
	}

	// Instant Raycast-style bare PX <-> REM flip ("24px" -> "1.5 rem")
	if m := reBareCSS.FindStringSubmatch(q); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		if strings.EqualFold(m[2], "px") {
			res := cleanFloat(v / baseRem)
			return fmt.Sprintf("%s rem", rawNumber(res)), rawNumber(res) + "rem", fmt.Sprintf("CSS conversion (base %.0fpx)", baseRem), true
		}
		res := cleanFloat(v * baseRem)
		return fmt.Sprintf("%s px", rawNumber(res)), rawNumber(res) + "px", fmt.Sprintf("CSS conversion (base %.0fpx)", baseRem), true
	}

	// Percentage ratio ("40 as % of 200" or "3.2 USD as % of 10 USD")
	if m := reRatioPct.FindStringSubmatch(q); m != nil {
		pLeft := newExprParser(m[1], baseRem)
		left, err1 := pLeft.evaluate()
		pRight := newExprParser(m[2], baseRem)
		pRight.rates = pLeft.rates
		right, err2 := pRight.evaluate()
		if err1 == nil && err2 == nil {
			if left.kind == right.kind && left.kind != kindScalar {
				if conv, err := pLeft.convert(right, left.kind, left.unit); err == nil {
					right = conv
				}
			}
			if right.val != 0 && (left.kind == right.kind || left.kind == kindScalar || right.kind == kindScalar) {
				pct := cleanFloat((left.val / right.val) * 100.0)
				return fmt.Sprintf("%s%%", formatNumber(pct)), rawNumber(pct), "Percentage ratio", true
			}
		}
	}

	p := newExprParser(q, baseRem)
	res, err := p.evaluate()
	if err != nil || math.IsNaN(res.val) || math.IsInf(res.val, 0) {
		return "", "", "", false
	}

	return p.formatQuantity(res)
}

// --- Lexer ---

type tokType int

const (
	tokEOF tokType = iota
	tokNum
	tokIdent
	tokSymCurr
	tokPlus
	tokMinus
	tokMul
	tokDiv
	tokCaret
	tokPercent
	tokLParen
	tokRParen
)

type token struct {
	typ tokType
	num float64
	str string
}

var symCurrencies = map[rune]string{
	'$': "USD",
	'₹': "INR",
	'€': "EUR",
	'£': "GBP",
	'¥': "JPY",
}

func tokenize(s string) ([]token, error) {
	s = reSqrtNL.ReplaceAllString(s, "sqrt")
	runes := []rune(s)
	n := len(runes)
	var toks []token

	for i := 0; i < n; {
		r := runes[i]
		if unicode.IsSpace(r) {
			i++
			continue
		}

		// Number literal (supports commas like 1,000.50 and 'k' shorthand like 7.8k)
		if unicode.IsDigit(r) || (r == '.' && i+1 < n && unicode.IsDigit(runes[i+1])) {
			var b strings.Builder
			dots := 0
			for i < n {
				c := runes[i]
				if unicode.IsDigit(c) {
					b.WriteRune(c)
					i++
				} else if c == ',' && i+1 < n && unicode.IsDigit(runes[i+1]) {
					i++ // skip grouping comma
				} else if c == '.' {
					dots++
					if dots > 1 {
						return nil, errors.New("invalid decimal")
					}
					b.WriteRune(c)
					i++
				} else {
					break
				}
			}
			val, err := strconv.ParseFloat(b.String(), 64)
			if err != nil {
				return nil, err
			}
			// Handle 'k' multiplier (e.g. 7.8k -> 7800), ensuring it is not part of a word like "km" or "kg"
			if i < n && (runes[i] == 'k' || runes[i] == 'K') {
				if i+1 == n || !unicode.IsLetter(runes[i+1]) {
					val *= 1000.0
					i++
				}
			}
			toks = append(toks, token{typ: tokNum, num: val})
			continue
		}

		if code, ok := symCurrencies[r]; ok {
			toks = append(toks, token{typ: tokSymCurr, str: code})
			i++
			continue
		}

		if unicode.IsLetter(r) {
			start := i
			for i < n && unicode.IsLetter(runes[i]) {
				i++
			}
			toks = append(toks, token{typ: tokIdent, str: string(runes[start:i])})
			continue
		}

		switch r {
		case '+':
			toks = append(toks, token{typ: tokPlus})
		case '-':
			toks = append(toks, token{typ: tokMinus})
		case '*', '×':
			if r == '*' && i+1 < n && runes[i+1] == '*' {
				toks = append(toks, token{typ: tokCaret})
				i++
			} else {
				toks = append(toks, token{typ: tokMul})
			}
		case '/', '÷':
			toks = append(toks, token{typ: tokDiv})
		case '^':
			toks = append(toks, token{typ: tokCaret})
		case '%':
			toks = append(toks, token{typ: tokPercent})
		case '(':
			toks = append(toks, token{typ: tokLParen})
		case ')':
			toks = append(toks, token{typ: tokRParen})
		default:
			return nil, fmt.Errorf("unexpected character: %c", r)
		}
		i++
	}

	toks = append(toks, token{typ: tokEOF})
	return toks, nil
}

// --- Parser & Unit-Aware Evaluator ---

type exprParser struct {
	input      string
	toks       []token
	pos        int
	baseRem    float64
	rates      map[string]float64
	staleRates bool
	rateSource string
}

func newExprParser(input string, baseRem float64) *exprParser {
	return &exprParser{input: input, baseRem: baseRem}
}

func (p *exprParser) peek() token {
	if p.pos >= len(p.toks) {
		return token{typ: tokEOF}
	}
	return p.toks[p.pos]
}

func (p *exprParser) peekAt(offset int) token {
	idx := p.pos + offset
	if idx < 0 || idx >= len(p.toks) {
		return token{typ: tokEOF}
	}
	return p.toks[idx]
}

func (p *exprParser) evaluate() (quantity, error) {
	toks, err := tokenize(p.input)
	if err != nil {
		return quantity{}, err
	}
	p.toks = toks

	q, err := p.parseExpr()
	if err != nil {
		return quantity{}, err
	}
	if p.peek().typ != tokEOF {
		return quantity{}, errors.New("unexpected trailing input")
	}

	// Auto-convert bare foreign currency ("59 GBP" -> INR) using DEFAULT_CURRENCY (defaults to INR)
	if len(p.toks) == 3 && q.kind == kindCurrency && !q.explicit {
		defCurr := strings.ToUpper(strings.TrimSpace(os.Getenv("DEFAULT_CURRENCY")))
		if defCurr == "" {
			defCurr = "INR"
		}
		if q.unit != defCurr {
			if conv, err := p.convert(q, kindCurrency, defCurr); err == nil {
				return conv, nil
			}
		}
	}

	return q, nil
}

func (p *exprParser) isConvWord(t token) bool {
	if t.typ != tokIdent {
		return false
	}
	switch strings.ToLower(t.str) {
	case "in", "to", "as":
		return true
	}
	return false
}

func isReservedKeyword(s string) bool {
	switch strings.ToLower(s) {
	case "in", "to", "as", "of", "off", "power",
		"pi", "e", "sqrt", "abs", "round", "ceil", "floor", "sin", "cos", "tan", "log", "ln":
		return true
	}
	return false
}

func resolveTimeUnit(s string) (string, float64, bool) {
	switch strings.ToLower(s) {
	case "h", "hr", "hrs", "hour", "hours":
		return "h", 3600.0, true
	case "m", "min", "mins", "minute", "minutes":
		return "m", 60.0, true
	case "s", "sec", "secs", "second", "seconds":
		return "s", 1.0, true
	case "timespan":
		return "timespan", 1.0, true
	}
	return "", 0, false
}

func (p *exprParser) resolveCSSUnit(s string) (string, float64, bool) {
	switch strings.ToLower(s) {
	case "px":
		return "px", 1.0, true
	case "rem":
		return "rem", p.baseRem, true
	case "em":
		return "em", p.baseRem, true
	case "pt":
		return "pt", 96.0 / 72.0, true
	}
	return "", 0, false
}

func (p *exprParser) resolveUnitToken(t token, allowTimespan bool) (unitKind, string, bool) {
	if t.typ == tokSymCurr {
		if err := p.ensureRates(); err == nil {
			if _, ok := p.rates[t.str]; ok {
				return kindCurrency, t.str, true
			}
		}
		return 0, "", false
	}
	if t.typ != tokIdent || isReservedKeyword(t.str) {
		return 0, "", false
	}
	if u, _, ok := resolveTimeUnit(t.str); ok {
		if u == "timespan" && !allowTimespan {
			return 0, "", false
		}
		return kindTime, u, true
	}
	if u, _, ok := p.resolveCSSUnit(t.str); ok {
		return kindCSS, u, true
	}
	if len(t.str) == 3 {
		if err := p.ensureRates(); err == nil {
			code := strings.ToUpper(t.str)
			if _, ok := p.rates[code]; ok {
				return kindCurrency, code, true
			}
		}
	}
	return 0, "", false
}

func (p *exprParser) parseExpr() (quantity, error) {
	q, err := p.parseAddSub()
	if err != nil {
		return quantity{}, err
	}
	// Handle expression-level conversion, e.g. "50 USD + 10 in INR"
	for p.isConvWord(p.peek()) {
		p.pos++
		targetTok := p.peek()
		kind, unit, ok := p.resolveUnitToken(targetTok, true)
		if !ok {
			return quantity{}, errors.New("unknown target unit")
		}
		p.pos++
		q, err = p.convert(q, kind, unit)
		if err != nil {
			return quantity{}, err
		}
	}
	return q, nil
}

func (p *exprParser) parseAddSub() (quantity, error) {
	left, err := p.parseTerm()
	if err != nil {
		return quantity{}, err
	}

	for p.peek().typ == tokPlus || p.peek().typ == tokMinus {
		op := p.peek().typ
		p.pos++
		right, err := p.parseTerm()
		if err != nil {
			return quantity{}, err
		}
		left, err = p.applyAddSub(left, op, right)
		if err != nil {
			return quantity{}, err
		}
	}
	return left, nil
}

func (p *exprParser) parseTerm() (quantity, error) {
	q, err := p.parseMulDiv()
	if err != nil {
		return quantity{}, err
	}

	// Term-level inline conversion, e.g. "32/10 USD in INR" inside "32/10 USD in INR + 100"
	// Skips if q is scalar so "50 USD + 10 in INR" converts the entire sum at parseExpr() level.
	for q.kind != kindScalar && p.isConvWord(p.peek()) {
		p.pos++
		targetTok := p.peek()
		kind, unit, ok := p.resolveUnitToken(targetTok, true)
		if !ok {
			return quantity{}, errors.New("unknown target unit")
		}
		p.pos++
		q, err = p.convert(q, kind, unit)
		if err != nil {
			return quantity{}, err
		}
	}
	return q, nil
}

func (p *exprParser) parseMulDiv() (quantity, error) {
	left, err := p.parsePower()
	if err != nil {
		return quantity{}, err
	}

	for {
		t := p.peek()
		isOf := t.typ == tokIdent && strings.EqualFold(t.str, "of")
		isOff := t.typ == tokIdent && strings.EqualFold(t.str, "off")
		if t.typ != tokMul && t.typ != tokDiv && !isOf && !isOff {
			break
		}
		p.pos++
		right, err := p.parsePower()
		if err != nil {
			return quantity{}, err
		}

		if isOff {
			if !left.isPercent {
				return quantity{}, errors.New("'off' requires a percentage on the left")
			}
			left = quantity{
				val:      (1.0 - left.val) * right.val,
				kind:     right.kind,
				unit:     right.unit,
				explicit: right.explicit,
			}
			continue
		}

		op := t.typ
		if isOf {
			op = tokMul
		}
		left, err = p.applyMulDiv(left, op, right)
		if err != nil {
			return quantity{}, err
		}
	}
	return left, nil
}

func (p *exprParser) parsePower() (quantity, error) {
	base, err := p.parseUnary()
	if err != nil {
		return quantity{}, err
	}
	t := p.peek()
	if t.typ == tokCaret || (t.typ == tokIdent && strings.EqualFold(t.str, "power")) {
		p.pos++
		exp, err := p.parsePower()
		if err != nil {
			return quantity{}, err
		}
		if exp.kind != kindScalar {
			return quantity{}, errors.New("exponent must be dimensionless")
		}
		return quantity{val: math.Pow(base.val, exp.val), kind: base.kind, unit: base.unit}, nil
	}
	return base, nil
}

func (p *exprParser) parseUnary() (quantity, error) {
	t := p.peek()
	if t.typ == tokPlus || t.typ == tokMinus {
		p.pos++
		q, err := p.parseUnary()
		if err != nil {
			return quantity{}, err
		}
		if t.typ == tokMinus {
			q.val = -q.val
		}
		return q, nil
	}
	return p.parseAtomWithUnit()
}

func (p *exprParser) parseAtomWithUnit() (quantity, error) {
	t := p.peek()

	// Prefix currency symbol ("$100", "₹500", "$(32/10)")
	if t.typ == tokSymCurr {
		if err := p.ensureRates(); err != nil {
			return quantity{}, err
		}
		code := t.str
		p.pos++
		inner, err := p.parsePrimary()
		if err != nil {
			return quantity{}, err
		}
		if inner.kind != kindScalar {
			return quantity{}, errors.New("conflicting units")
		}
		return quantity{val: inner.val, kind: kindCurrency, unit: code}, nil
	}

	q, err := p.parsePrimary()
	if err != nil {
		return quantity{}, err
	}

	// Postfix % ("18%", "(10+8)%")
	if p.peek().typ == tokPercent && q.kind == kindScalar {
		p.pos++
		return quantity{val: q.val / 100.0, kind: kindScalar, isPercent: true}, nil
	}

	// Postfix unit attachment ("10 USD", "(32/10) USD", "3h 40m", "24px")
	if q.kind == kindScalar {
		if kind, unit, ok := p.resolveUnitToken(p.peek(), false); ok {
			p.pos++
			if kind == kindTime {
				_, factor, _ := resolveTimeUnit(unit)
				totalSec := q.val * factor
				compound := false

				// Absorb compound base-60 literals like "3h 40m 15s"
				for p.peek().typ == tokNum {
					if nextKind, nextU, ok2 := p.resolveUnitToken(p.peekAt(1), false); ok2 && nextKind == kindTime {
						_, nextFactor, _ := resolveTimeUnit(nextU)
						totalSec += p.peek().num * nextFactor
						p.pos += 2
						compound = true
					} else {
						break
					}
				}
				if compound {
					return quantity{val: totalSec, kind: kindTime, unit: "timespan"}, nil
				}
				return quantity{val: q.val, kind: kindTime, unit: unit}, nil
			}
			return quantity{val: q.val, kind: kind, unit: unit}, nil
		}
	}

	return q, nil
}

func (p *exprParser) parsePrimary() (quantity, error) {
	t := p.peek()

	if t.typ == tokLParen {
		p.pos++
		q, err := p.parseExpr()
		if err != nil {
			return quantity{}, err
		}
		if p.peek().typ != tokRParen {
			return quantity{}, errors.New("missing closing parenthesis")
		}
		p.pos++
		return q, nil
	}

	if t.typ == tokIdent {
		name := strings.ToLower(t.str)
		if name == "pi" {
			p.pos++
			return quantity{val: math.Pi, kind: kindScalar}, nil
		}
		if name == "e" {
			p.pos++
			return quantity{val: math.E, kind: kindScalar}, nil
		}

		switch name {
		case "sqrt", "abs", "round", "ceil", "floor", "sin", "cos", "tan", "log", "ln":
			p.pos++
			var arg quantity
			var err error
			if p.peek().typ == tokLParen {
				p.pos++
				arg, err = p.parseExpr()
				if err != nil {
					return quantity{}, err
				}
				if p.peek().typ != tokRParen {
					return quantity{}, errors.New("missing ')'")
				}
				p.pos++
			} else {
				arg, err = p.parseUnary()
				if err != nil {
					return quantity{}, err
				}
			}

			var out float64
			switch name {
			case "sqrt":
				out = math.Sqrt(arg.val)
			case "abs":
				out = math.Abs(arg.val)
			case "round":
				out = math.Round(arg.val)
			case "ceil":
				out = math.Ceil(arg.val)
			case "floor":
				out = math.Floor(arg.val)
			case "sin":
				out = math.Sin(arg.val)
			case "cos":
				out = math.Cos(arg.val)
			case "tan":
				out = math.Tan(arg.val)
			case "log":
				out = math.Log10(arg.val)
			case "ln":
				out = math.Log(arg.val)
			}
			// Preserve unit for rounding/abs helpers (e.g., "round(32/10 USD in INR)")
			if name == "abs" || name == "round" || name == "ceil" || name == "floor" {
				return quantity{val: out, kind: arg.kind, unit: arg.unit, explicit: arg.explicit}, nil
			}
			if arg.kind != kindScalar {
				return quantity{}, errors.New("trig/log requires dimensionless value")
			}
			return quantity{val: out, kind: kindScalar}, nil
		}
	}

	if t.typ == tokNum {
		p.pos++
		return quantity{val: t.num, kind: kindScalar}, nil
	}

	return quantity{}, errors.New("expected value")
}

// --- Unit Arithmetic & Conversion Rules ---

func (p *exprParser) convert(q quantity, targetKind unitKind, targetUnit string) (quantity, error) {
	if q.kind == kindScalar {
		q.kind = targetKind
		q.unit = targetUnit
		q.explicit = true
		return q, nil
	}
	if q.kind != targetKind {
		return quantity{}, errors.New("incompatible unit conversion")
	}

	switch targetKind {
	case kindCurrency:
		if err := p.ensureRates(); err != nil {
			return quantity{}, err
		}
		rFrom := p.rates[q.unit]
		rTo := p.rates[targetUnit]
		if rFrom <= 0 || rTo <= 0 {
			return quantity{}, errors.New("missing currency rate")
		}
		return quantity{
			val:      (q.val / rFrom) * rTo,
			kind:     kindCurrency,
			unit:     targetUnit,
			explicit: true,
		}, nil

	case kindTime:
		_, fFrom, _ := resolveTimeUnit(q.unit)
		_, fTo, _ := resolveTimeUnit(targetUnit)
		secs := q.val * fFrom
		return quantity{
			val:      secs / fTo,
			kind:     kindTime,
			unit:     targetUnit,
			explicit: true,
		}, nil

	case kindCSS:
		_, fFrom, _ := p.resolveCSSUnit(q.unit)
		_, fTo, _ := p.resolveCSSUnit(targetUnit)
		px := q.val * fFrom
		return quantity{
			val:      px / fTo,
			kind:     kindCSS,
			unit:     targetUnit,
			explicit: true,
		}, nil
	}
	return quantity{}, errors.New("unsupported conversion")
}

func (p *exprParser) applyAddSub(left quantity, op tokType, right quantity) (quantity, error) {
	sign := 1.0
	if op == tokMinus {
		sign = -1.0
	}

	// Raycast percentage +/- ("1000 + 18%", "100 USD in INR + 18%")
	if right.isPercent && !left.isPercent {
		return quantity{
			val:      left.val * (1.0 + sign*right.val),
			kind:     left.kind,
			unit:     left.unit,
			explicit: left.explicit,
		}, nil
	}

	if left.kind == kindScalar && right.kind == kindScalar {
		return quantity{
			val:       left.val + sign*right.val,
			kind:      kindScalar,
			isPercent: left.isPercent && right.isPercent,
		}, nil
	}

	// Scalar + Unit or Unit + Scalar ("32/10 USD in INR + 100" -> 403.83 INR)
	if left.kind != kindScalar && right.kind == kindScalar {
		return quantity{
			val:      left.val + sign*right.val,
			kind:     left.kind,
			unit:     left.unit,
			explicit: left.explicit,
		}, nil
	}
	if left.kind == kindScalar && right.kind != kindScalar {
		return quantity{
			val:      left.val + sign*right.val,
			kind:     right.kind,
			unit:     right.unit,
			explicit: right.explicit,
		}, nil
	}

	if left.kind != right.kind {
		return quantity{}, errors.New("cannot add mismatched unit types")
	}

	// Adding two durations defaults to formatted timespan ("40m + 20m" -> "1h") unless explicitly converted
	if left.kind == kindTime && !left.explicit && !right.explicit {
		lTs, _ := p.convert(left, kindTime, "timespan")
		rTs, _ := p.convert(right, kindTime, "timespan")
		return quantity{val: lTs.val + sign*rTs.val, kind: kindTime, unit: "timespan"}, nil
	}

	// If right-hand term had an explicit "in <UNIT>", convert left to match right ("10 USD + 20 EUR in INR")
	if right.explicit && !left.explicit {
		lConv, err := p.convert(left, right.kind, right.unit)
		if err != nil {
			return quantity{}, err
		}
		return quantity{
			val:      lConv.val + sign*right.val,
			kind:     right.kind,
			unit:     right.unit,
			explicit: true,
		}, nil
	}

	rConv, err := p.convert(right, left.kind, left.unit)
	if err != nil {
		return quantity{}, err
	}
	return quantity{
		val:      left.val + sign*rConv.val,
		kind:     left.kind,
		unit:     left.unit,
		explicit: left.explicit,
	}, nil
}

func (p *exprParser) applyMulDiv(left quantity, op tokType, right quantity) (quantity, error) {
	if op == tokDiv && right.val == 0 {
		return quantity{}, errors.New("division by zero")
	}

	if left.kind == kindScalar && right.kind == kindScalar {
		if op == tokMul {
			return quantity{val: left.val * right.val, kind: kindScalar}, nil
		}
		return quantity{val: left.val / right.val, kind: kindScalar}, nil
	}

	// Supports both "100 USD / 4" AND Raycast-style "32/10 USD" -> 3.2 USD
	if left.kind != kindScalar && right.kind == kindScalar {
		if op == tokMul {
			return quantity{val: left.val * right.val, kind: left.kind, unit: left.unit, explicit: left.explicit}, nil
		}
		return quantity{val: left.val / right.val, kind: left.kind, unit: left.unit, explicit: left.explicit}, nil
	}
	if left.kind == kindScalar && right.kind != kindScalar {
		if op == tokMul {
			return quantity{val: left.val * right.val, kind: right.kind, unit: right.unit, explicit: right.explicit}, nil
		}
		return quantity{val: left.val / right.val, kind: right.kind, unit: right.unit, explicit: right.explicit}, nil
	}

	// Ratio of two same-dimension quantities ("1000 INR / 10 USD" or "3h / 30m") -> dimensionless scalar
	if op == tokDiv && left.kind == right.kind {
		rConv, err := p.convert(right, left.kind, left.unit)
		if err != nil || rConv.val == 0 {
			return quantity{}, errors.New("invalid ratio")
		}
		return quantity{val: left.val / rConv.val, kind: kindScalar}, nil
	}

	return quantity{}, errors.New("unsupported unit multiplication")
}

// --- Currency Rate Loader (Reads Official Alfred Cache -> ExchangeRate-API Fallback) ---

type diskRateCache struct {
	UpdatedAt int64              `json:"updated_at"`
	Rates     map[string]float64 `json:"rates"`
}

func (p *exprParser) ensureRates() error {
	if len(p.rates) > 0 {
		return nil
	}

	// 1. Check Alfred's official Currency Converter cache first so rates match 'cur' 1:1
	if official, ok := readOfficialAlfredCurrencyCache(); ok {
		p.rates = official
		p.rateSource = "Alfred Currency Converter cache"
		return nil
	}

	// 2. Check our own 12-hour ExchangeRate-API disk cache
	cacheDir := os.Getenv("alfred_workflow_cache")
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "alfred_raycalc_cache")
	}
	_ = os.MkdirAll(cacheDir, 0755)
	cacheFile := filepath.Join(cacheDir, "er_rates.json")

	var cached diskRateCache
	hasStale := false
	if b, err := os.ReadFile(cacheFile); err == nil {
		if json.Unmarshal(b, &cached) == nil && len(cached.Rates) > 0 {
			hasStale = true
			if time.Since(time.Unix(cached.UpdatedAt, 0)) < 12*time.Hour {
				p.rates = cached.Rates
				p.rateSource = "ExchangeRate-API"
				return nil
			}
		}
	}

	// 3. Fetch fresh rates from ExchangeRate-API (same provider as Alfred's official workflow)
	client := &http.Client{Timeout: 2500 * time.Millisecond}
	endpoints := []string{
		"https://open.er-api.com/v6/latest/USD",
		"https://api.frankfurter.dev/v1/latest?base=USD",
	}

	for _, url := range endpoints {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		var payload struct {
			Rates map[string]float64 `json:"rates"`
		}
		err = json.NewDecoder(resp.Body).Decode(&payload)
		_ = resp.Body.Close()
		if err == nil && len(payload.Rates) > 0 {
			norm := make(map[string]float64, len(payload.Rates)+1)
			norm["USD"] = 1.0
			for k, v := range payload.Rates {
				norm[strings.ToUpper(k)] = v
			}
			_ = saveRateCache(cacheFile, norm)
			p.rates = norm
			p.rateSource = "ExchangeRate-API"
			return nil
		}
	}

	if hasStale {
		p.rates = cached.Rates
		p.staleRates = true
		p.rateSource = "offline cached rate"
		return nil
	}

	return errors.New("currency rates unavailable")
}

func readOfficialAlfredCurrencyCache() (map[string]float64, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, false
	}
	wfDataRoot := filepath.Join(home, "Library", "Caches", "com.runningwithcrayons.Alfred", "Workflow Data")
	if envCache := os.Getenv("alfred_workflow_cache"); envCache != "" {
		wfDataRoot = filepath.Dir(envCache)
	}

	candidates := []string{
		filepath.Join(wfDataRoot, "com.alfredapp.vitor.currencyconverter"),
		filepath.Join(wfDataRoot, "com.alfredapp.alfredteam.currencyconverter"),
	}

	for _, dir := range candidates {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			fullPath := filepath.Join(dir, entry.Name())
			info, err := os.Stat(fullPath)
			if err != nil || time.Since(info.ModTime()) > 24*time.Hour {
				continue
			}
			data, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}
			if parsed, ok := extractRatesMap(data); ok {
				return parsed, true
			}
		}
	}
	return nil, false
}

func extractRatesMap(data []byte) (map[string]float64, bool) {
	var wrapped struct {
		Rates           map[string]float64 `json:"rates"`
		ConversionRates map[string]float64 `json:"conversion_rates"`
	}
	if json.Unmarshal(data, &wrapped) == nil {
		src := wrapped.Rates
		if len(src) == 0 {
			src = wrapped.ConversionRates
		}
		if len(src) > 10 {
			out := make(map[string]float64, len(src))
			for k, v := range src {
				out[strings.ToUpper(k)] = v
			}
			if out["USD"] > 0 {
				return out, true
			}
		}
	}
	var flat map[string]float64
	if json.Unmarshal(data, &flat) == nil && len(flat) > 10 {
		out := make(map[string]float64, len(flat))
		for k, v := range flat {
			out[strings.ToUpper(k)] = v
		}
		if out["USD"] > 0 {
			return out, true
		}
	}
	return nil, false
}

func saveRateCache(path string, rates map[string]float64) error {
	b, err := json.Marshal(diskRateCache{UpdatedAt: time.Now().Unix(), Rates: rates})
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// --- Output Formatting ---

func (p *exprParser) formatQuantity(q quantity) (string, string, string, bool) {
	switch q.kind {
	case kindCurrency:
		decimals := 2
		if math.Abs(q.val) > 0 && math.Abs(q.val) < 1 {
			decimals = 4
		}
		rounded := math.Round(q.val*math.Pow10(decimals)) / math.Pow10(decimals)
		formatted := fmt.Sprintf("%s %s", formatCurrencyNum(rounded, decimals), q.unit)
		raw := strconv.FormatFloat(rounded, 'f', decimals, 64)
		desc := fmt.Sprintf("Currency math (%s)", p.rateSource)

		// Show quick home-currency preview in subtitle if result is still in USD/EUR/GBP
		defCurr := strings.ToUpper(strings.TrimSpace(os.Getenv("DEFAULT_CURRENCY")))
		if defCurr == "" {
			defCurr = "INR"
		}
		if q.unit != defCurr {
			if preview, err := p.convert(q, kindCurrency, defCurr); err == nil {
				pRound := math.Round(preview.val*100) / 100
				desc = fmt.Sprintf("≈ %s %s  •  %s", formatCurrencyNum(pRound, 2), defCurr, p.rateSource)
			}
		}
		return formatted, raw, desc, true

	case kindTime:
		if q.unit == "timespan" {
			ts := formatTimespan(q.val)
			desc := fmt.Sprintf("Time math: %.0f mins  |  %.2f hrs", q.val/60, q.val/3600)
			return ts, ts, desc, true
		}
		_, factor, _ := resolveTimeUnit(q.unit)
		secs := q.val * factor
		valClean := cleanFloat(q.val)
		labels := map[string]string{"h": "hours", "m": "mins", "s": "secs"}
		return fmt.Sprintf("%s %s (%s)", formatNumber(valClean), labels[q.unit], formatTimespan(secs)), rawNumber(valClean), "Duration calculation", true

	case kindCSS:
		valClean := cleanFloat(q.val)
		raw := rawNumber(valClean) + q.unit
		return fmt.Sprintf("%s %s", formatNumber(valClean), q.unit), raw, fmt.Sprintf("CSS math (base %.0fpx)", p.baseRem), true

	default:
		if q.isPercent {
			pct := cleanFloat(q.val * 100.0)
			return fmt.Sprintf("%s%%", formatNumber(pct)), rawNumber(pct), "Percentage calculation", true
		}
		valClean := cleanFloat(q.val)
		return formatNumber(valClean), rawNumber(valClean), "Math calculation", true
	}
}

func formatTimespan(totalSec float64) string {
	neg := totalSec < 0
	absSec := int(math.Round(math.Abs(totalSec)))
	if absSec == 0 {
		return "0m"
	}
	h := absSec / 3600
	m := (absSec % 3600) / 60
	s := absSec % 60

	var parts []string
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
	}
	if s > 0 {
		parts = append(parts, fmt.Sprintf("%ds", s))
	}
	res := strings.Join(parts, " ")
	if neg {
		return "-" + res
	}
	return res
}

func cleanFloat(v float64) float64 {
	if math.Abs(v) < 1e12 && math.Abs(v) >= 1e-6 {
		return math.Round(v*1e8) / 1e8
	}
	return v
}

func rawNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func formatCurrencyNum(v float64, decimals int) string {
	raw := strconv.FormatFloat(v, 'f', decimals, 64)
	return addThousandsCommas(raw)
}

func formatNumber(v float64) string {
	raw := strconv.FormatFloat(v, 'f', -1, 64)
	parts := strings.Split(raw, ".")
	if len(parts) == 2 && len(parts[1]) > 6 {
		dec := strings.TrimRight(parts[1][:6], "0")
		if dec != "" {
			raw = parts[0] + "." + dec
		} else {
			raw = parts[0]
		}
	}
	return addThousandsCommas(raw)
}

func addThousandsCommas(raw string) string {
	parts := strings.Split(raw, ".")
	intPart := parts[0]
	neg := false
	if strings.HasPrefix(intPart, "-") {
		neg = true
		intPart = intPart[1:]
	}

	var b strings.Builder
	n := len(intPart)
	for i, ch := range intPart {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}

	res := b.String()
	if neg {
		res = "-" + res
	}
	if len(parts) == 2 {
		res += "." + parts[1]
	}
	return res
}
