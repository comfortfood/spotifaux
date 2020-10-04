package spotifaux

type Winner struct {
	File    string `json:"file"`
	Winner  int    `json:"winner"`
	MinDist float64
}

type Recipe struct {
	Winner []Winner `json:"recipe"`
}
