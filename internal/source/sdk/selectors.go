package sdk

// s.dk (mit.s.dk/studiebolig) portal endpoints and selectors. s.dk is a Django
// app fronted by a Vue SPA: a plain POST login form, then async-rendered lists.
const (
	baseURL  = "https://mit.s.dk"
	loginURL = "https://mit.s.dk/studiebolig/login/"
	listURL  = "https://mit.s.dk/studiebolig/home/"

	selLoginUser     = "#id_username"
	selLoginPassword = "#id_password"
	selLoginSubmit   = "#id_login"
	selCookieAccept  = ".cc-accept"

	// The list page renders each org as a collapsed accordion whose property
	// list-group only populates once expanded; long lists are paginated behind a
	// "Vis flere ejendomme" button that must be clicked until exhausted.
	selAccordionToggle = "a.group-toggle-link"
	txtShowMore        = "vis flere ejendomme"

	// Each property links to its building detail page; that page holds a per
	// tenancy ranking. selRankCell marks a tenancy the applicant joined (present
	// whether ranked or not); selRankLetter holds the waiting-list letter A–G,
	// absent until s.dk calculates the position.
	selBuildingLink = `.list-group a.list-group-item[href*="/studiebolig/building/"]`
	selRankCell     = ".waiting-list-category-label"
	selRankLetter   = ".waiting-list-category"
)
