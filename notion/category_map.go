package notion

var categoryIDToName map[string]string

func InitCategoryMap() {
	cats, fallback := SeedCategories()

	categoryIDToName = make(map[string]string)
	for _, c := range cats {
		categoryIDToName[c.ID] = c.Name
	}

	// Ensure fallback is present
	categoryIDToName[fallback.ID] = fallback.Name
}

func ResolveCategoryName(id string) string {
	if name, ok := categoryIDToName[id]; ok {
		return name
	}
	return "Miscellaneous"
}
