package libgobuster

// Result represents a single gobuster result
type Result struct {
	Entity      string
	Status      int
	Extra       string
	Size        *int64
	Content     *string
	IsEntityURL bool
	RedirectURL *string
}

// ToString converts the Result to it's textual representation
func (r *Result) ToString(g *Gobuster) (string, string, int, error) {
	s, as, status, err := g.plugin.ResultToString(g, r)
	if err != nil {
		return "", "", 0, err
	}
	return *s, *as, status, nil
}
