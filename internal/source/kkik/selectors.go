package kkik

// KKIK portal endpoints and login-form selectors.
const (
	baseURL    = "https://www.kollegierneskontor.dk"
	loginURL   = "https://www.kollegierneskontor.dk/default.aspx?func=kkikportal.login&lang=GB"
	housingURL = "https://www.kollegierneskontor.dk/default.aspx?func=kkikportal.housingrequests&mid=10&topmenuid=5&lang=GB"

	selLoginUser     = "#Page_ctl08_Main_ctl04_form_loginUserName"
	selLoginPassword = "#Page_ctl08_Main_ctl04_form_loginPassword"
	selLoginSubmit   = "#Page_ctl08_Main_ctl04_form_loginButton"
	selHousingMarker = "div.func-housingrequests"
)
