package resource

import "strings"

// siteSystemLabels are the $Label.site.* system labels a Visualforce page can
// reference, in the en_US language. The list and values come from the native
// rows in internal/visualforce/testdata/vf_site_labels (API 59.0 through 68.0
// are identical); names that deploy natively as "does not exist" are absent.
var siteSystemLabels = map[string]string{
	"address_information":                  "Address Information",
	"authorization_required":               "Authorization Required",
	"bandwidth_limit_exceeded":             "Bandwidth Limit Exceeded",
	"cancel":                               "Cancel",
	"change_password":                      "Change Password",
	"change_your_password":                 "Change Your Password",
	"click_forget_password":                "If you have forgotten your password, click Forgot Password to reset it.",
	"community_nickname":                   "Nickname",
	"confirm_password":                     "Confirm Password",
	"contact_information":                  "Contact Information",
	"down_for_maintenance":                 "<i>{0}</i> is down for maintenance",
	"edit":                                 "Edit",
	"email":                                "Email",
	"email_us":                             "email us",
	"enter_password":                       "Did you forget your password? Please enter your username below.",
	"error":                                "Error: {0}",
	"error2":                               "Error",
	"file_not_found":                       "File Not Found",
	"forgot_password":                      "Forgot Password",
	"forgot_password_confirmation":         "Forgot Password Confirmation",
	"forgot_your_password_q":               "Forgot Your Password?",
	"get_in_touch":                         "Please <a href=\"{0}\">{1}</a> if you need to get in touch.",
	"go_to_login_page":                     "Go to Login Page",
	"img_path":                             "/img/sites",
	"in_maintenance":                       "Down For Maintenance",
	"invalid_email":                        "Invalid Email Address",
	"limit_exceeded":                       "Limit Exceeded",
	"login":                                "Login",
	"login_button":                         "Login",
	"login_or_register_first":              "You must first log in or register before accessing this page.",
	"logout":                               "Logout",
	"my_profile":                           "My Profile",
	"my_profile_page":                      "My Profile Page",
	"new_password":                         "New Password",
	"new_user_q":                           "New User?",
	"old_password":                         "Old Password",
	"page_not_found":                       "Page Not Found",
	"page_not_found_detail":                "Page Not Found: {0}",
	"password":                             "Password",
	"password_too_short":                   "Password too short.",
	"passwords_dont_match":                 "Passwords did not match.",
	"powered_by":                           "Powered by",
	"register":                             "Register",
	"registration_confirmation":            "Registration Confirmation",
	"save":                                 "Save",
	"site_login":                           "Site Login",
	"site_under_construction":              "Site Under Construction",
	"sorry_for_inconvenience":              "Sorry for the inconvenience.",
	"sorry_for_inconvenience_back_shortly": "Sorry for the inconvenience. We'll be back shortly.",
	"stay_tuned":                           "Stay tuned.",
	"submit":                               "Submit",
	"temp_password_sent":                   "An email has been sent to you with your temporary password.",
	"thank_you_for_registering":            "Thank you for registering. An email has been sent to you with your temporary password.",
	"thank_you_for_submitting_registration_request": "Thanks for submitting your registration request! We’ll send you an email with further instructions once your request has finished processing.",
	"under_construction":                            "<i>{0}</i> is under construction",
	"user_information":                              "User Information",
	"user_registration":                             "New User Registration",
	"username":                                      "Username",
	"verify_new_password":                           "Verify New Password",
}

// SiteSystemLabel returns the text of a $Label.site.* system label. Names are
// case-insensitive.
func SiteSystemLabel(name string) (string, bool) {
	value, ok := siteSystemLabels[strings.ToLower(strings.TrimSpace(name))]
	return value, ok
}
