package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/noahjalex/epoch/internal/auth"
	"github.com/noahjalex/epoch/internal/logging"
	"github.com/noahjalex/epoch/internal/middleware"
	"github.com/noahjalex/epoch/internal/models"
	"github.com/noahjalex/epoch/internal/utils"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

type Server struct {
	rend      *Renderer
	repo      *models.Repo
	log       *logrus.Logger
	logConfig *logging.Config
}

func NewServer(repo *models.Repo, log *logrus.Logger, logConfig *logging.Config) (*Server, error) {
	rend, err := NewRendererWithLogger(log)
	if err != nil {
		return nil, err
	}

	return &Server{rend: rend, repo: repo, log: log, logConfig: logConfig}, nil
}

func (server *Server) Run(port string) error {
	open := false

	mux := http.NewServeMux()

	// Create a file server to serve files from the "static" directory
	fs := http.FileServer(http.Dir("./static"))
	// Handle requests for "/static/" by stripping the prefix and serving files
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Apply auth middleware to ALL routes (including auth pages)
	// The middleware will handle the logic for auth vs protected pages
	allRoutes := http.NewServeMux()

	// Auth routes - these will be handled by middleware but allowed through
	allRoutes.HandleFunc("GET /login", server.handleLoginPage)
	allRoutes.HandleFunc("POST /login", server.handleLogin)
	allRoutes.HandleFunc("GET /signup", server.handleSignupPage)
	allRoutes.HandleFunc("POST /signup", server.handleSignup)
	allRoutes.HandleFunc("POST /logout", server.handleLogout)

	// Protected routes
	allRoutes.HandleFunc("/", server.handleHome)

	// API routes
	allRoutes.HandleFunc("GET /api/habits", server.handleHabitsListAPI)
	allRoutes.HandleFunc("POST /api/habits", server.handleHabitCreateAPI)
	allRoutes.HandleFunc("PATCH /api/habits/{id}", server.handleHabitUpdateAPI)
	allRoutes.HandleFunc("DELETE /api/habits/{id}", server.handleHabitDeleteAPI)
	allRoutes.HandleFunc("GET /api/logs", server.handleLogsListAPI)
	allRoutes.HandleFunc("POST /api/logs", server.handleLogCreateAPI)
	allRoutes.HandleFunc("PATCH /api/logs/{id}", server.handleLogUpdateAPI)
	allRoutes.HandleFunc("DELETE /api/logs/{id}", server.handleLogDeleteAPI)

	// Apply middleware in order: Request ID -> HTTP Logging -> Auth
	var handler http.Handler = allRoutes

	// Apply auth middleware first (innermost)
	handler = middleware.AuthMiddleware(server.repo, server.log)(handler)

	// Apply HTTP logging middleware if enabled
	if server.logConfig.HTTPLogging {
		handler = LoggingMiddleware(server.log)(handler)
	}

	// Apply request ID middleware (outermost)
	handler = middleware.RequestIDMiddleware()(handler)

	mux.Handle("/", handler)

	if open {
		openServer(port)
	}

	server.log.WithField("port", port).Info("HTTP server listening")
	return http.ListenAndServe(port, mux)
}

func (app *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := middleware.GetRequestIDFromContext(ctx)

	user, ok := middleware.GetUserFromContext(ctx)
	if !ok {
		app.log.WithFields(logrus.Fields{
			"component":  "handler",
			"action":     "home",
			"request_id": requestID,
		}).Debug("No authenticated user found in context, redirecting to login")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	app.log.WithFields(logrus.Fields{
		"component":  "handler",
		"action":     "home",
		"user_id":    user.ID,
		"username":   user.Username,
		"request_id": requestID,
	}).Debug("Loading home page for authenticated user")

	habits, err := app.repo.ListHabitsByUser(ctx, user.ID, true)
	if err != nil {
		app.log.WithFields(logrus.Fields{
			"component":  "handler",
			"action":     "home",
			"user_id":    user.ID,
			"username":   user.Username,
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Database query failed while fetching user habits with details")
		http.Error(w, "Failed to load your habits", http.StatusInternalServerError)
		return
	}

	app.log.WithFields(logrus.Fields{
		"component":   "handler",
		"action":      "home",
		"user_id":     user.ID,
		"username":    user.Username,
		"request_id":  requestID,
		"habit_count": len(habits),
	}).Info("Successfully loaded home page with user habits")

	data := struct {
		Habits     []models.Habit
		IsAuthPage bool
	}{
		Habits:     habits,
		IsAuthPage: false,
	}

	app.rend.Render(w, "home", data)
}

// writeNoContent returns 204 StatusNoContent and no resource
func writeNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// writeCreated returns 201 StatusCreated and a JSON resource
func writeCreated(w http.ResponseWriter, resource any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if resource != nil {
		_ = json.NewEncoder(w).Encode(resource)
	}
}

func openServer(port string) {
	go func() {
		time.Sleep(3 * time.Second)
		cmd := exec.Command("open", fmt.Sprintf("http://localhost%s", port))
		if err := cmd.Run(); err != nil {
			return
		}
	}()
}

func getVars(r *http.Request, i int) string {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) <= i {
		return ""
	}
	return parts[i]
}

func getQuery(r *http.Request, name string) string {
	val := r.URL.Query().Get(name)
	if val == "" {
		return ""
	}
	return val
}

// Frontend data models (simplified for demo compatibility)
type FrontendHabit struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Unit string  `json:"unit"`
	Goal float64 `json:"goal"`
}

type FrontendLog struct {
	ID          string  `json:"id"`
	HabitID     string  `json:"habitId"`
	Date        string  `json:"date"`
	DateDisplay string  `json:"date_display"`
	Qty         float64 `json:"qty"`
}

// Data transformation functions
func habitToFrontend(h *models.Habit) FrontendHabit {
	unit := ""
	if h.UnitLabel.Valid {
		unit = h.UnitLabel.String
	}

	goal, _ := h.TargetPerPeriod.Float64()

	return FrontendHabit{
		ID:   fmt.Sprintf("%d", h.ID),
		Name: h.Name,
		Unit: unit,
		Goal: goal,
	}
}

func logToFrontend(l *models.HabitLog, userTZ *time.Location) FrontendLog {
	qty, _ := l.Quantity.Float64()

	occurredAtInUserTZ := l.OccurredAt.In(userTZ)

	return FrontendLog{
		ID:          fmt.Sprintf("%d", l.ID),
		HabitID:     fmt.Sprintf("%d", l.HabitID),
		Date:        occurredAtInUserTZ.Format(models.ToFrontEndFormat),
		DateDisplay: occurredAtInUserTZ.Format(models.HumanDateFormat),
		Qty:         qty,
	}
}

func (app *Server) handleHabitsListAPI(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := middleware.GetUserFromContext(ctx)
	if !ok {
		// Redirect to login
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	habits, err := app.repo.ListHabitsByUser(ctx, user.ID, true)
	if err != nil {
		app.log.WithError(err).Error("Failed to get habits")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Transform to frontend format
	frontendHabits := make([]FrontendHabit, len(habits))
	for i, h := range habits {
		frontendHabits[i] = habitToFrontend(&h)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(frontendHabits)
}

func (app *Server) handleHabitCreateAPI(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := middleware.GetRequestIDFromContext(ctx)

	user, ok := middleware.GetUserFromContext(ctx)
	if !ok {
		app.log.WithFields(logrus.Fields{
			"component":  "api",
			"action":     "habit_create",
			"request_id": requestID,
		}).Warn("Unauthenticated API request to create habit")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var req FrontendHabit
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.log.WithFields(logrus.Fields{
			"component":  "api",
			"action":     "habit_create",
			"user_id":    user.ID,
			"username":   user.Username,
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to decode JSON request body for habit creation")
		http.Error(w, "Invalid JSON request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Name == "" {
		app.log.WithFields(logrus.Fields{
			"component":  "api",
			"action":     "habit_create",
			"user_id":    user.ID,
			"username":   user.Username,
			"request_id": requestID,
		}).Warn("Habit creation attempted with empty name")
		http.Error(w, "Habit name is required", http.StatusBadRequest)
		return
	}

	app.log.WithFields(logrus.Fields{
		"component":  "api",
		"action":     "habit_create",
		"user_id":    user.ID,
		"username":   user.Username,
		"request_id": requestID,
		"habit_name": req.Name,
		"habit_unit": req.Unit,
		"habit_goal": req.Goal,
	}).Info("Creating new habit for user")

	// Transform to backend format
	habit := &models.Habit{
		UserID:           user.ID,
		Name:             req.Name,
		UnitLabel:        sql.NullString{String: req.Unit, Valid: req.Unit != ""},
		Agg:              models.AggSum,
		TargetPerPeriod:  decimal.NewFromFloat(req.Goal),
		PerLogDefaultQty: decimal.NewFromFloat(1),
		Period:           models.PeriodDaily,
		WeekStartDOW:     1, // Monday
		MonthAnchorDay:   1,
		AnchorDate:       time.Now(),
		IsActive:         true,
	}

	createdHabit, err := app.repo.CreateHabit(ctx, habit)
	if err != nil {
		app.log.WithFields(logrus.Fields{
			"component":  "api",
			"action":     "habit_create",
			"user_id":    user.ID,
			"username":   user.Username,
			"request_id": requestID,
			"habit_name": req.Name,
			"error":      err.Error(),
		}).Error("Database error while creating habit")
		http.Error(w, "Failed to create habit", http.StatusInternalServerError)
		return
	}

	app.log.WithFields(logrus.Fields{
		"component":  "api",
		"action":     "habit_create",
		"user_id":    user.ID,
		"username":   user.Username,
		"request_id": requestID,
		"habit_id":   createdHabit.ID,
		"habit_name": createdHabit.Name,
	}).Info("Successfully created new habit")

	frontendHabit := habitToFrontend(createdHabit)
	writeCreated(w, frontendHabit)
}

func (app *Server) handleHabitUpdateAPI(w http.ResponseWriter, r *http.Request) {
	habitIDStr := r.PathValue("id")
	ctx := r.Context()

	habitID, err := strconv.ParseInt(habitIDStr, 10, 64)
	if err != nil {
		app.log.WithError(err).Error("Invalid habit ID")
		http.Error(w, "Invalid habit ID", http.StatusBadRequest)
		return
	}

	var req FrontendHabit
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.log.WithError(err).Error("Failed to decode request JSON")
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	habit, err := app.repo.GetHabit(ctx, habitID)
	if err != nil {
		app.log.WithError(err).Error("Habit not found")
		http.Error(w, "Habit not found", http.StatusNotFound)
		return
	}

	// Update fields
	habit.Name = req.Name
	habit.UnitLabel = sql.NullString{String: req.Unit, Valid: req.Unit != ""}
	habit.TargetPerPeriod = decimal.NewFromFloat(req.Goal)

	err = app.repo.UpdateHabit(ctx, habit)
	if err != nil {
		app.log.WithError(err).Error("Failed to update habit")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	frontendHabit := habitToFrontend(habit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(frontendHabit)
}

func (app *Server) handleHabitDeleteAPI(w http.ResponseWriter, r *http.Request) {
	habitIDStr := r.PathValue("id")
	ctx := r.Context()

	habitID, err := strconv.ParseInt(habitIDStr, 10, 64)
	if err != nil {
		app.log.WithError(err).Error("Invalid habit ID")
		http.Error(w, "Invalid habit ID", http.StatusBadRequest)
		return
	}

	err = app.repo.DeleteHabit(ctx, habitID)
	if err != nil {
		app.log.WithError(err).Error("Failed to delete habit")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeNoContent(w)
}

func (app *Server) handleLogsListAPI(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := middleware.GetUserFromContext(ctx)
	if !ok {
		// Redirect to login
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Get all habits for this user to filter logs
	habits, err := app.repo.ListHabitsByUser(ctx, user.ID, false)
	if err != nil {
		app.log.WithError(err).Error("Failed to get habits")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var allLogs []models.HabitLog
	for _, habit := range habits {
		logs, err := app.repo.ListLogs(ctx, habit.ID)
		if err != nil {
			app.log.WithError(err).Error("Failed to get logs")
			continue
		}
		allLogs = append(allLogs, logs...)
	}

	loc, _ := time.LoadLocation(user.TZ)

	// Transform to frontend format
	frontendLogs := make([]FrontendLog, len(allLogs))
	for i, l := range allLogs {
		frontendLogs[i] = logToFrontend(&l, loc)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(frontendLogs)
}

func (app *Server) handleLogCreateAPI(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := middleware.GetUserFromContext(ctx)
	if !ok {
		// Redirect to login
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var req FrontendLog
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.log.WithError(err).Error("Failed to decode request JSON")
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	habitID, err := strconv.ParseInt(req.HabitID, 10, 64)
	if err != nil {
		app.log.WithError(err).Error("Invalid habit ID")
		http.Error(w, "Invalid habit ID", http.StatusBadRequest)
		return
	}

	loc, _ := time.LoadLocation(user.TZ)

	occurredAt, err := time.ParseInLocation(models.ToFrontEndFormat, req.Date, loc)
	if err != nil {
		app.log.WithError(err).Error("Invalid date format")
		http.Error(w, "Invalid date format", http.StatusBadRequest)
		return
	}
	log := &models.HabitLog{
		HabitID:    habitID,
		OccurredAt: occurredAt.UTC(),
		Quantity:   decimal.NewFromFloat(req.Qty),
	}

	createdLog, err := app.repo.InsertLog(ctx, log)
	if err != nil {
		app.log.WithError(err).Error("Failed to create log")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	frontendLog := logToFrontend(createdLog, loc)
	writeCreated(w, frontendLog)
}

func (app *Server) handleLogUpdateAPI(w http.ResponseWriter, r *http.Request) {
	logIDStr := r.PathValue("id")
	ctx := r.Context()
	user, ok := middleware.GetUserFromContext(ctx)
	if !ok {
		// Redirect to login
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	logID, err := strconv.ParseInt(logIDStr, 10, 64)
	if err != nil {
		app.log.WithError(err).Error("Invalid log ID")
		http.Error(w, "Invalid log ID", http.StatusBadRequest)
		return
	}

	var req FrontendLog
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.log.WithError(err).Error("Failed to decode request JSON")
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	habitID, err := strconv.ParseInt(req.HabitID, 10, 64)
	if err != nil {
		app.log.WithError(err).Error("Invalid habit ID")
		http.Error(w, "Invalid habit ID", http.StatusBadRequest)
		return
	}

	loc, _ := time.LoadLocation(user.TZ)
	occurredAt, err := time.ParseInLocation(models.ToFrontEndFormat, req.Date, loc)
	if err != nil {
		app.log.WithError(err).Error("Invalid date format")
		http.Error(w, "Invalid date format", http.StatusBadRequest)
		return
	}

	log := &models.HabitLog{
		ID:         logID,
		HabitID:    habitID,
		OccurredAt: occurredAt,
		Quantity:   decimal.NewFromFloat(req.Qty),
	}

	err = app.repo.UpdateLog(ctx, log)
	if err != nil {
		app.log.WithError(err).Error("Failed to update log")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	frontendLog := logToFrontend(log, loc)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(frontendLog)
}

func (app *Server) handleLogDeleteAPI(w http.ResponseWriter, r *http.Request) {
	logIDStr := r.PathValue("id")
	ctx := r.Context()

	logID, err := strconv.ParseInt(logIDStr, 10, 64)
	if err != nil {
		app.log.WithError(err).Error("Invalid log ID")
		http.Error(w, "Invalid log ID", http.StatusBadRequest)
		return
	}

	err = app.repo.DeleteLog(ctx, logID)
	if err != nil {
		app.log.WithError(err).Error("Failed to delete log")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeNoContent(w)
}

// ======= Authentication Handlers =======

func (app *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := middleware.GetUserFromContext(ctx)
	if ok && user != nil {
		app.log.Debug("Redirecting authenticated user from login page to home")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	data := struct {
		IsAuthPage bool
		Error      string
		Username   string
	}{
		IsAuthPage: true,
	}
	app.rend.Render(w, "login", data)
}

func (app *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fx := utils.New(r)
	username := fx.String("username", utils.Required())
	password := fx.String("password", utils.Required())

	if err := fx.Err(); err != nil {
		data := struct {
			IsAuthPage bool
			Error      string
			Username   string
		}{
			IsAuthPage: true,
			Error:      "Username and password are required",
			Username:   username,
		}
		app.rend.Render(w, "login", data)
		return
	}

	// Get user by username
	user, err := app.repo.GetUserByUsername(r.Context(), username)
	if err != nil {
		if err == sql.ErrNoRows {
			data := struct {
				IsAuthPage bool
				Error      string
				Username   string
			}{
				IsAuthPage: true,
				Error:      "Invalid username or password",
				Username:   username,
			}
			app.rend.Render(w, "login", data)
			return
		}
		app.log.WithError(err).Error("Failed to get user")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check password
	if !auth.CheckPassword(password, user.PasswordHash) {
		data := struct {
			IsAuthPage bool
			Error      string
			Username   string
		}{
			IsAuthPage: true,
			Error:      "Invalid username or password",
			Username:   username,
		}
		app.rend.Render(w, "login", data)
		return
	}

	// Create session
	sessionToken, err := auth.GenerateSessionToken()
	if err != nil {
		app.log.WithError(err).Error("Failed to generate session token")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	expiresAt := auth.GetSessionExpiry()
	_, err = app.repo.CreateSession(r.Context(), user.ID, sessionToken, expiresAt)
	if err != nil {
		app.log.WithError(err).Error("Failed to create session")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set session cookie
	middleware.SetSessionCookie(w, sessionToken)

	// Redirect to home
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *Server) handleSignupPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, ok := middleware.GetUserFromContext(ctx)
	if ok {
		// Redirect to home page, this is a logged in user
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	data := struct {
		IsAuthPage bool
		Error      string
		Username   string
		Email      string
	}{
		IsAuthPage: true,
	}
	app.rend.Render(w, "signup", data)
}

func (app *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fx := utils.New(r)
	username := fx.String("username", utils.Required())
	email := fx.String("email", utils.Required())
	password := fx.String("password", utils.Required())
	confirmPassword := fx.String("confirm_password", utils.Required())
	timezone := fx.String("timezone") // optional

	if err := fx.Err(); err != nil {
		data := struct {
			IsAuthPage bool
			Error      string
			Username   string
			Email      string
		}{
			IsAuthPage: true,
			Error:      "All fields are required",
			Username:   username,
			Email:      email,
		}
		app.rend.Render(w, "signup", data)
		return
	}

	// Validate passwords match
	if password != confirmPassword {
		data := struct {
			IsAuthPage bool
			Error      string
			Username   string
			Email      string
		}{
			IsAuthPage: true,
			Error:      "Passwords do not match",
			Username:   username,
			Email:      email,
		}
		app.rend.Render(w, "signup", data)
		return
	}

	// Check if username already exists
	_, err := app.repo.GetUserByUsername(r.Context(), username)
	if err == nil {
		data := struct {
			IsAuthPage bool
			Error      string
			Username   string
			Email      string
		}{
			IsAuthPage: true,
			Error:      "Username already exists",
			Username:   username,
			Email:      email,
		}
		app.rend.Render(w, "signup", data)
		return
	} else if err != sql.ErrNoRows {
		app.log.WithError(err).Error("Failed to check username")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Hash password
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		app.log.WithError(err).Error("Failed to hash password")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Create user
	user, err := app.repo.CreateUser(r.Context(), username, email, passwordHash, timezone)
	if err != nil {
		app.log.WithError(err).Error("Failed to create user")
		data := struct {
			IsAuthPage bool
			Error      string
			Username   string
			Email      string
		}{
			IsAuthPage: true,
			Error:      "Failed to create account. Username or email may already exist.",
			Username:   username,
			Email:      email,
		}
		app.rend.Render(w, "signup", data)
		return
	}

	// Create session
	sessionToken, err := auth.GenerateSessionToken()
	if err != nil {
		app.log.WithError(err).Error("Failed to generate session token")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	expiresAt := auth.GetSessionExpiry()
	_, err = app.repo.CreateSession(r.Context(), user.ID, sessionToken, expiresAt)
	if err != nil {
		app.log.WithError(err).Error("Failed to create session")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set session cookie
	middleware.SetSessionCookie(w, sessionToken)

	// Redirect to home
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session token from cookie
	if c, err := r.Cookie("session_token"); err == nil && c.Value != "" {
		_ = app.repo.DeleteSession(r.Context(), c.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/", // match original Path
		Domain:   "",  // set if you originally set it
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode, // match original SameSite
		Secure:   false,                // Match the setting in SetSessionCookie
		MaxAge:   -1,                   // expire immediately (don’t rely on Expires alone)
	})

	w.Header().Set("Cache-Control", "no-store")

	w.WriteHeader(http.StatusNoContent) // 204
}
