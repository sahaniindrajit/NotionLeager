package expense

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
)

type Expense struct {
	Name        string
	Amount      float64
	CategoryRaw string
	Description string
}

func Parse(text string) (*Expense, error) {
	parts := strings.Split(text, ",")

	//Trim
	clean := make([]string, 0)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			clean = append(clean, p)
		}
	}

	if len(clean) < 3 {
		return nil, errors.New("Invalid Formate")
	}

	amount, err := parseAmount(clean[1])
	if err != nil {
		return nil, errors.New("invalid amount")
	}

	exp := &Expense{
		Name:        clean[0],
		Amount:      amount,
		CategoryRaw: clean[2],
	}

	if len(clean) >= 4 {
		exp.Description = clean[3]
	}

	return exp, nil
}

func parseAmount(input string) (float64, error) {

	var b strings.Builder

	for _, r := range input {
		if unicode.IsDigit(r) || r == '.' {
			b.WriteRune(r)
		}
	}

	return strconv.ParseFloat(b.String(), 64)
}
