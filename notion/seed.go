package notion

func SeedCategories() ([]Category, Category) {
	categories := []Category{
		{ID: "0e694186-2d9a-491c-ad11-e9a2684abe10", Name: "Business"},
		{ID: "17ea2834-b809-4bf7-8b53-7b5c766db92b", Name: "Shopping"},
		{ID: "183267cf-08f9-4bdc-aa1d-54bf2e027814", Name: "Entertainment"},
		{ID: "28478c44-adcc-80c7-9907-d5c44206e742", Name: "Miscellaneous"},
		{ID: "2f278c44-adcc-801f-9427-f3dd1b55e291", Name: "Donation"},
		{ID: "2f278c44-adcc-80eb-9547-e948a27acf6b", Name: "Leager"},
		{ID: "47a3c7b1-ec42-414d-b9df-f0b8ffe11ef6", Name: "Travel"},
		{ID: "48da7a44-30d1-4420-8b42-06de1a085516", Name: "Food & Drink"},
		{ID: "5b116fef-9d86-4ad0-9580-3f28d31ca8e7", Name: "Education"},
		{ID: "64d0df09-2ba4-4efd-a366-baf0b177a62f", Name: "Home & Utility"},
		{ID: "a87e0be9-7773-4bd7-bff7-0fca6b55429a", Name: "Health & Supplements"},
		{ID: "ac58fe2f-92a4-4d67-90f8-c6ff3e5b8a8a", Name: "Subscription"},
		{ID: "ef3e05d4-9fa4-47af-b5af-633a831177af", Name: "Insurance"},
		{ID: "f5f8a050-260f-4591-981b-a8600ca7f072", Name: "Family"},
		{ID: "fb386d97-1b46-42ba-87e7-a492ed406bc3", Name: "Emi"},
	}

	fallback := Category{
		ID:   "28478c44-adcc-80c7-9907-d5c44206e742",
		Name: "Miscellaneous",
	}

	return categories, fallback
}
