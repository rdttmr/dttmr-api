package request

type CreateRecipePayload struct {
	Name string `json:"name"`
}

type SetRecipeNamePayload struct {
	Name string `json:"name"`
}

type OrderRecipesPayload struct {
	RecipeIDs []string `json:"recipe_ids"`
}

type AddListItemToRecipePayload struct {
	RecipeID   string `json:"recipe_id"`
	ListItemID string `json:"list_item_id"`
}

type RemoveListItemFromRecipePayload struct {
	RecipeID   string `json:"recipe_id"`
	ListItemID string `json:"list_item_id"`
}
