package request

type CreateRecipePayload struct {
	Name    string `json:"name"`
	GroupID string `json:"group_id"`
}

type SetRecipeGroupPayload struct {
	GroupID string `json:"group_id"`
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
