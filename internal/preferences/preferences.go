package preferences

import "encoding/json"

// user preferences stored as jsonb in database,
// onto users table so that users queries can select preferences without another join
type UserPreferences struct {
	EmailNotifications bool `json:"email_notifications"`
}

// default preferences go here
var DefaultPreferences = UserPreferences{
	EmailNotifications: true,
}

// function to merge bytes (body) into userpreferences defaults so that
// new preferences can be added safely
// (probably needs tests)
func Merge(stored []byte) (UserPreferences, error) {
	prefs := DefaultPreferences
	if len(stored) == 0 || string(stored) == "{}" {
		return prefs, nil
	}
	if err := json.Unmarshal(stored, &prefs); err != nil {
		return DefaultPreferences, err
	}
	return prefs, nil
}
