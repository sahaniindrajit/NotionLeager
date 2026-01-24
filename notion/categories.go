package notion

import "strings"

type Category struct {
	ID   string
	Name string
}

type CategoryResolver struct {
	categories []Category
	fallback   Category
}

func NewCategoryResolver(categories []Category, fallback Category) *CategoryResolver {

	return &CategoryResolver{
		categories: categories,
		fallback:   fallback,
	}
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "&", "and")
	return s
}

func (r *CategoryResolver) Resolve(input string) Category {
	in := normalize(input)

	// 1️⃣ Exact match
	for _, c := range r.categories {
		if normalize(c.Name) == in {
			return c
		}
	}

	// 2️⃣ Substring match
	for _, c := range r.categories {
		n := normalize(c.Name)
		if strings.Contains(n, in) || strings.Contains(in, n) {
			return c
		}
	}

	// 3️⃣ Fallback
	return r.fallback
}
