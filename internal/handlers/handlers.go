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

	"github.com/noahjalex/epoch/internal/models"
	"github.com/noahjalex/epoch/internal/utils"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

type Server struct {
	rend *Renderer
	repo *models.Repo
	log  *logrus.Logger
}

func NewServer(repo *models.Repo, log *logrus.Logger) (*Server, error) {
	rend, err := NewRenderer()
	if err != nil {
		return nil, err
	}

	return &Server{rend: rend, repo: repo, log: log}, nil
}

func (server *Server) Run(port string) {
	open := false

	mux := http.NewServeMux()

	// Create a file server to serve files from the "static" directory
	fs := http.FileServer(http.Dir("./static"))
	// Handle requests for "/static/" by stripping the prefix and serving files
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Web routes
	mux.HandleFunc("/", server.handleHome)
	mux.HandleFunc("GET /logs/create", server.handleLogCreateForm)

	// JSON API endpoints for frontend
	mux.HandleFunc("GET /api/habits", server.handleHabitsListAPI)
	mux.HandleFunc("POST /api/habits", server.handleHabitCreateAPI)
	mux.HandleFunc("PATCH /api/habits/{id}", server.handleHabitUpdateAPI)
	mux.HandleFunc("DELETE /api/habits/{id}", server.handleHabitDeleteAPI)
	mux.HandleFunc("GET /api/logs", server.handleLogsListAPI)
	mux.HandleFunc("POST /api/logs", server.handleLogCreateAPI)
	mux.HandleFunc("PATCH /api/logs/{id}", server.handleLogUpdateAPI)
	mux.HandleFunc("DELETE /api/logs/{id}", server.handleLogDeleteAPI)

	if open {
		openServer(port)
	}
	err := http.ListenAndServe(port, mux)
	if err != nil {
		panic(err)
	}
}

func (app *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	userID := int64(1) // Hardcoded for demo
	ctx := r.Context()
	habits, err := app.repo.ListHabitsByUser(ctx, userID, true)

	if err != nil {
		app.log.WithError(err).Error("Failed to get habits with details")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	app.rend.Render(w, "home", habits)
}

func (app *Server) handleLogCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := int64(1) // hardcoded for demo

	if r.Method != http.MethodPost {
		app.log.Error("Invalid HTTP method")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user to access their timezone
	user, err := app.repo.GetUser(ctx, userID)
	if err != nil {
		app.log.WithError(err).Error("No user found")
		http.Error(w, "User not found", http.StatusBadRequest)
		return
	}

	fx := utils.New(r)

	habitID := fx.Int64("habit_id", utils.Required(), utils.MinInt(1))
	occurredAtStr := fx.String("occurred_at", utils.Required())
	quantity := fx.Float64("quantity", utils.Required(), utils.MinFloat(0))
	note := fx.String("note") // optional

	// Parse the datetime in user's timezone, then convert to UTC
	userTZ, err := time.LoadLocation(user.TZ)
	if err != nil {
		app.log.WithError(err).Error("Invalid user timezone")
		userTZ = time.Local // fallback to local timezone
	}

	var occurredAt time.Time
	if occurredAtStr != "" {
		// Parse in user's timezone
		occurredAt, err = time.ParseInLocation("2006-01-02T15:04", occurredAtStr, userTZ)
		if err != nil {
			app.log.WithError(err).Error("Invalid datetime format")
			http.Error(w, "Invalid datetime format", http.StatusBadRequest)
			return
		}
		// Convert to UTC for storage
		occurredAt = occurredAt.UTC()
	} else {
		occurredAt = time.Now().UTC()
	}

	req := &models.HabitLog{
		HabitID:    habitID,
		OccurredAt: occurredAt,
		Quantity:   decimal.NewFromFloat(quantity),
		Note:       sql.NullString{String: note, Valid: note != ""},
	}

	// Verify habit is one of User's actual habits
	habits, err := app.repo.ListHabitsByUser(ctx, userID, false)
	if err != nil {
		app.log.WithError(err).Error("Failed to return habits")
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}
	var isHabit bool = false
	for _, h := range habits {
		if h.ID == req.HabitID {
			isHabit = true
		}
	}
	if isHabit != true {
		app.log.WithError(err).Error("Failed to return habits")
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	habitLog, err := app.repo.InsertLog(ctx, req)
	if err != nil {
		app.log.WithError(err).Error("Failed to create habit log")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	habitURL := fmt.Sprintf("/habits/%d", habitLog.HabitID)

	if r.Header.Get("HX-Request") == "true" {
		// htmx request: instruct client to do a full navigation
		w.Header().Set("HX-Redirect", habitURL)
		w.WriteHeader(http.StatusSeeOther) // 303 is fine; 200 also works
		return
	}

	app.rend.Render(w, "habit", habitLog)
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
	userID := int64(1) // Hardcoded for demo
	ctx := r.Context()

	habits, err := app.repo.ListHabitsByUser(ctx, userID, true)
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
	userID := int64(1) // Hardcoded for demo
	ctx := r.Context()

	var req FrontendHabit
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.log.WithError(err).Error("Failed to decode request JSON")
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Transform to backend format
	habit := &models.Habit{
		UserID:           userID,
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
		app.log.WithError(err).Error("Failed to create habit")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
	userID := int64(1) // Hardcoded for demo
	ctx := r.Context()

	user, err := app.repo.GetUser(ctx, userID)
	if err != nil {
		app.log.WithError(err).Error("No user found")
		http.Error(w, "User not found", http.StatusBadRequest)
		return
	}

	// Get all habits for this user to filter logs
	habits, err := app.repo.ListHabitsByUser(ctx, userID, false)
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
	userID := int64(1)
	ctx := r.Context()

	user, err := app.repo.GetUser(ctx, userID)
	if err != nil {
		app.log.WithError(err).Error("No user found")
		http.Error(w, "User not found", http.StatusBadRequest)
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
	app.log.Infof("createdLog: %+v\n", frontendLog)
	writeCreated(w, frontendLog)
}

func (app *Server) handleLogUpdateAPI(w http.ResponseWriter, r *http.Request) {
	logIDStr := r.PathValue("id")
	userID := int64(1)
	ctx := r.Context()

	user, err := app.repo.GetUser(ctx, userID)
	if err != nil {
		app.log.WithError(err).Error("No user found")
		http.Error(w, "User not found", http.StatusBadRequest)
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

func (app *Server) handleLogCreateForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	habitIDStr := r.URL.Query().Get("habit_id")

	var err error
	var habitID int64
	if habitIDStr != "" {
		habitID, err = strconv.ParseInt(habitIDStr, 10, 64)
		if err != nil {
			app.log.WithError(err).Error("couldn't parse habitID")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	userID := int64(1) // hardcoded for demo

	// Get user to access their timezone
	user, err := app.repo.GetUser(ctx, userID)
	if err != nil {
		app.log.WithError(err).Error("No user found")
		http.Error(w, "User not found", http.StatusBadRequest)
		return
	}

	habits, err := app.repo.ListHabitsByUser(ctx, userID, true)
	if err != nil {
		app.log.Warn("Habits not found")
		http.Error(w, "No habits found", http.StatusNotFound)
		return
	}

	// Get current time in user's timezone
	userTZ, err := time.LoadLocation(user.TZ)
	if err != nil {
		app.log.WithError(err).Error("Invalid user timezone")
		userTZ = time.Local // fallback to local timezone
	}

	currentTimeInUserTZ := time.Now().In(userTZ).Format("2006-01-02T15:04")

	data := struct {
		Habits        []models.Habit
		SelectedHabit int64
		CurrentTime   string
	}{
		Habits:        habits,
		SelectedHabit: habitID,
		CurrentTime:   currentTimeInUserTZ,
	}
	app.rend.RenderPartial(w, "log-create", data)
}
