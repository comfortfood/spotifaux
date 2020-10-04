package spotifaux

type Winner struct {
	Filename string `json:"filename"`
	Winner   int    `json:"winner"`
	MinDist  float64
}

type Recipe struct {
	Winner []Winner `json:"recipe"`
}
