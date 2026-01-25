package notion

import (
	"strings"
)

type Category struct {
	ID   string
	Name string
}

type CategoryResolver struct {
	categories []Category
	fallback   Category
	aliases    map[string]string // keyword → category name
}

func NewCategoryResolver(categories []Category, fallback Category) *CategoryResolver {
	r := &CategoryResolver{
		categories: categories,
		fallback:   fallback,
		aliases:    buildAliases(),
	}
	return r
}

// buildAliases maps common keywords to category names
func buildAliases() map[string]string {
	return map[string]string{
		// Food & Drink
		"food":       "Food & Drink",
		"lunch":      "Food & Drink",
		"dinner":     "Food & Drink",
		"breakfast":  "Food & Drink",
		"cafe":       "Food & Drink",
		"restaurant": "Food & Drink",
		"coffee":     "Food & Drink",
		"tea":        "Food & Drink",
		"snack":      "Food & Drink",
		"meal":       "Food & Drink",
		"eat":        "Food & Drink",
		"drink":      "Food & Drink",
		"zomato":     "Food & Drink",
		"swiggy":     "Food & Drink",

		// Travel
		"uber":      "Travel",
		"ola":       "Travel",
		"cab":       "Travel",
		"taxi":      "Travel",
		"flight":    "Travel",
		"train":     "Travel",
		"bus":       "Travel",
		"metro":     "Travel",
		"petrol":    "Travel",
		"fuel":      "Travel",
		"parking":   "Travel",
		"toll":      "Travel",
		"rapido":    "Travel",
		"transport": "Travel",

		// Subscription
		"netflix":      "Subscription",
		"spotify":      "Subscription",
		"prime":        "Subscription",
		"youtube":      "Subscription",
		"hotstar":      "Subscription",
		"disney":       "Subscription",
		"hbo":          "Subscription",
		"apple":        "Subscription",
		"icloud":       "Subscription",
		"chatgpt":      "Subscription",
		"openai":       "Subscription",
		"claude":       "Subscription",
		"anthropic":    "Subscription",
		"github":       "Subscription",
		"copilot":      "Subscription",
		"subscription": "Subscription",
		"sub":          "Subscription",

		// Health & Supplements
		"gym":         "Health & Supplements",
		"medicine":    "Health & Supplements",
		"doctor":      "Health & Supplements",
		"hospital":    "Health & Supplements",
		"pharmacy":    "Health & Supplements",
		"medical":     "Health & Supplements",
		"health":      "Health & Supplements",
		"supplement":  "Health & Supplements",
		"vitamin":     "Health & Supplements",
		"protein":     "Health & Supplements",
		"fitness":     "Health & Supplements",
		"clinic":      "Health & Supplements",
		"checkup":     "Health & Supplements",
		"test":        "Health & Supplements",
		"pharmeasy":   "Health & Supplements",
		"1mg":         "Health & Supplements",
		"healthkart":  "Health & Supplements",

		// Shopping
		"amazon":   "Shopping",
		"flipkart": "Shopping",
		"myntra":   "Shopping",
		"ajio":     "Shopping",
		"shop":     "Shopping",
		"shopping": "Shopping",
		"clothes":  "Shopping",
		"shoes":    "Shopping",
		"gadget":   "Shopping",
		"meesho":   "Shopping",

		// Entertainment
		"movie":         "Entertainment",
		"cinema":        "Entertainment",
		"pvr":           "Entertainment",
		"inox":          "Entertainment",
		"game":          "Entertainment",
		"gaming":        "Entertainment",
		"concert":       "Entertainment",
		"event":         "Entertainment",
		"entertainment": "Entertainment",
		"ent":           "Entertainment",
		"fun":           "Entertainment",
		"party":         "Entertainment",

		// Home & Utility
		"electricity": "Home & Utility",
		"electric":    "Home & Utility",
		"water":       "Home & Utility",
		"gas":         "Home & Utility",
		"rent":        "Home & Utility",
		"wifi":        "Home & Utility",
		"internet":    "Home & Utility",
		"broadband":   "Home & Utility",
		"phone":       "Home & Utility",
		"mobile":      "Home & Utility",
		"recharge":    "Home & Utility",
		"bill":        "Home & Utility",
		"utility":     "Home & Utility",
		"home":        "Home & Utility",
		"maintenance": "Home & Utility",
		"repair":      "Home & Utility",

		// Education
		"course":    "Education",
		"book":      "Education",
		"books":     "Education",
		"udemy":     "Education",
		"coursera":  "Education",
		"class":     "Education",
		"tuition":   "Education",
		"education": "Education",
		"learn":     "Education",
		"study":     "Education",
		"school":    "Education",
		"college":   "Education",
		"exam":      "Education",

		// Insurance
		"insurance": "Insurance",
		"lic":       "Insurance",
		"policy":    "Insurance",
		"premium":   "Insurance",

		// Family
		"family": "Family",
		"mom":    "Family",
		"dad":    "Family",
		"parent": "Family",
		"gift":   "Family",

		// EMI
		"emi":  "Emi",
		"loan": "Emi",

		// Business
		"business": "Business",
		"office":   "Business",
		"work":     "Business",
		"client":   "Business",

		// Donation
		"donation": "Donation",
		"donate":   "Donation",
		"charity":  "Donation",
	}
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "&", "and")
	return s
}

func (r *CategoryResolver) Resolve(input string) Category {
	in := normalize(input)

	// 1. Exact match
	for _, c := range r.categories {
		if normalize(c.Name) == in {
			return c
		}
	}

	// 2. Alias match
	if catName, ok := r.aliases[in]; ok {
		for _, c := range r.categories {
			if c.Name == catName {
				return c
			}
		}
	}

	// 3. Prefix match (input is prefix of category name)
	if len(in) >= 3 {
		for _, c := range r.categories {
			n := normalize(c.Name)
			// Check first word prefix
			firstWord := strings.Split(n, " ")[0]
			if strings.HasPrefix(firstWord, in) {
				return c
			}
		}
	}

	// 4. Substring match
	for _, c := range r.categories {
		n := normalize(c.Name)
		if strings.Contains(n, in) || strings.Contains(in, n) {
			return c
		}
	}

	// 5. Fuzzy match (typo tolerance)
	if len(in) >= 3 {
		bestMatch := r.fallback
		bestScore := 3 // max allowed distance (threshold)

		for _, c := range r.categories {
			n := normalize(c.Name)
			// Check against full name and first word
			words := []string{n, strings.Split(n, " ")[0]}
			for _, word := range words {
				dist := levenshtein(in, word)
				// Allow distance proportional to word length (max ~30% errors)
				maxDist := len(word) / 3
				if maxDist < 2 {
					maxDist = 2
				}
				if dist < bestScore && dist <= maxDist {
					bestScore = dist
					bestMatch = c
				}
			}
		}

		// Also check aliases for fuzzy match
		for alias, catName := range r.aliases {
			dist := levenshtein(in, alias)
			maxDist := len(alias) / 3
			if maxDist < 2 {
				maxDist = 2
			}
			if dist < bestScore && dist <= maxDist {
				for _, c := range r.categories {
					if c.Name == catName {
						bestScore = dist
						bestMatch = c
						break
					}
				}
			}
		}

		if bestMatch.ID != r.fallback.ID {
			return bestMatch
		}
	}

	// 6. Fallback
	return r.fallback
}

// levenshtein calculates the edit distance between two strings
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	if len(a) > len(b) {
		a, b = b, a
	}

	prev := make([]int, len(a)+1)
	curr := make([]int, len(a)+1)

	for i := range prev {
		prev[i] = i
	}

	for j := 1; j <= len(b); j++ {
		curr[0] = j
		for i := 1; i <= len(a); i++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[i] = min(
				prev[i]+1,      // deletion
				curr[i-1]+1,    // insertion
				prev[i-1]+cost, // substitution
			)
		}
		prev, curr = curr, prev
	}

	return prev[len(a)]
}

func min(nums ...int) int {
	m := nums[0]
	for _, n := range nums[1:] {
		if n < m {
			m = n
		}
	}
	return m
}
