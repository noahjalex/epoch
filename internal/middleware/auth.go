package middleware

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/noahjalex/epoch/internal/auth"
	"github.com/noahjalex/epoch/internal/models"
	"github.com/sirupsen/logrus"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
)

// AuthMiddleware checks for a valid session and adds user to context
func AuthMiddleware(repo *models.Repo, log *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			isAuthPage := r.URL.Path == "/login" || r.URL.Path == "/signup"

			// Get session token from cookie
			cookie, err := r.Cookie("session_token")
			if err != nil {
				log.Debug("No session cookie found")
				// No session cookie
				if isAuthPage {
					// Allow access to auth pages without session
					next.ServeHTTP(w, r)
					return
				}
				// Redirect to login for protected pages
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			// Look up session in database
			session, err := repo.GetSessionByToken(r.Context(), cookie.Value)
			if err != nil {
				if err == sql.ErrNoRows {
					// Invalid session, clear cookie
					clearSessionCookie(w)
					if isAuthPage {
						// Allow access to auth pages with invalid session
						next.ServeHTTP(w, r)
						return
					}
					// Redirect to login for protected pages
					http.Redirect(w, r, "/login", http.StatusSeeOther)
					return
				}
				log.WithError(err).Error("Failed to get session")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			// Check if session is expired
			if auth.IsSessionExpired(session.ExpiresAt) {
				// Session expired, clean up
				_ = repo.DeleteSession(r.Context(), session.SessionToken)
				clearSessionCookie(w)
				if isAuthPage {
					// Allow access to auth pages with expired session
					next.ServeHTTP(w, r)
					return
				}
				// Redirect to login for protected pages
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			// Get user from session
			user, err := repo.GetUser(r.Context(), session.UserID)
			if err != nil {
				log.WithError(err).Error("Failed to get user from session")
				clearSessionCookie(w)
				if isAuthPage {
					// Allow access to auth pages if user lookup fails
					next.ServeHTTP(w, r)
					return
				}
				// Redirect to login for protected pages
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			// Add user to request context
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserFromContext extracts the user from the request context
func GetUserFromContext(ctx context.Context) (*models.AppUser, bool) {
	user, ok := ctx.Value(UserContextKey).(*models.AppUser)
	return user, ok
}

// SetSessionCookie sets the session cookie
func SetSessionCookie(w http.ResponseWriter, sessionToken string) {
	cookie := &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(auth.DefaultSessionDuration),
	}
	http.SetCookie(w, cookie)
}

// clearSessionCookie clears the session cookie
func clearSessionCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
	}
	http.SetCookie(w, cookie)
}
