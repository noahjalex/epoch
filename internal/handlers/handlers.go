package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/noahjalex/epoch/internal/models"
	"github.com/sirupsen/logrus"
)

type HomeData struct {
	Habits []models.HabitWithDetails `json:"habits"`
}

type Server struct {
	rend *Renderer
	repo *models.Repository
	log  *logrus.Logger
}

func NewServer(repo *models.Repository, log *logrus.Logger) (*Server, error) {
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
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Web routes
	mux.HandleFunc("/", server.handleHome)
	mux.HandleFunc("GET /habits/{id}", server.handleHabitView)
	mux.HandleFunc("GET /habits", server.handleHabitCreateForm)
	mux.HandleFunc("POST /habits", server.handleHabitCreate)

	// API routes
	// mux.HandleFunc("GET /api/habits", app.handleHabitsView)
	mux.HandleFunc("POST /api/habits/{id}/logs", server.handleLogCreate)
	// mux.HandleFunc("GET /api/logs", app.handleRecentLogsView)

	// // Web routes (server-rendered)
	// mux.HandleFunc("GET /", server.handleHome)
	// mux.HandleFunc("GET /habits/{slug}", server.handleHabitView)
	// mux.HandleFunc("GET /habits", server.handleHabitCreateForm)
	// mux.HandleFunc("POST /habits", server.handleHabitCreate)
	//
	// // API: habits
	// mux.HandleFunc("GET /api/habits", server.handleHabitsList)
	// mux.HandleFunc("POST /api/habits", server.handleHabitCreateAPI)
	// mux.HandleFunc("GET /api/habits/{slug}", server.handleHabitGet)
	// mux.HandleFunc("PATCH /api/habits/{slug}", server.handleHabitPatch)
	// mux.HandleFunc("DELETE /api/habits/{slug}", server.handleHabitDelete)
	//
	// // API: logs (scoped to habit)
	// mux.HandleFunc("GET /api/habits/{slug}/logs", server.handleHabitLogsList)
	// mux.HandleFunc("POST /api/habits/{slug}/logs", server.handleLogCreate)
	// mux.HandleFunc("GET /api/habits/{slug}/logs/{id}", server.handleLogGet)
	// mux.HandleFunc("PATCH /api/habits/{slug}/logs/{id}", server.handleLogPatch)
	// mux.HandleFunc("DELETE /api/habits/{slug}/logs/{id}", server.handleLogDelete)
	//
	// // Optional global logs endpoints
	// mux.HandleFunc("GET /api/logs", server.handleLogsSearch)
	// mux.HandleFunc("GET /api/logs/{id}", server.handleLogGetGlobal)

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
	habits, err := app.repo.GetHabitsWithDetails(userID)

	if err != nil {
		app.log.WithError(err).Error("Failed to get habits with details")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := &HomeData{Habits: habits}

	app.rend.Render(w, "home", data)
}

func (app *Server) handleHabitView(w http.ResponseWriter, r *http.Request) {
	habitIDStr := r.PathValue("id")
	userID := int64(1) // Hardcoded for demo

	habitID, err := strconv.ParseInt(habitIDStr, 10, 64)
	if err != nil {
		app.log.WithError(err).Error("Invalid habit ID provided")
		http.Error(w, "Invalid habit ID", http.StatusBadRequest)
		return
	}

	habits, err := app.repo.GetHabitsWithDetails(userID)
	if err != nil {
		app.log.WithError(err).Error("Failed to get habits with details")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var habit *models.HabitWithDetails
	for _, h := range habits {
		if h.Habit.ID == habitID {
			habit = &h
			break
		}
	}

	if habit == nil {
		app.log.Warn("Habit not found")
		http.Error(w, "Habit not found", http.StatusNotFound)
		return
	}

	app.rend.Render(w, "habit", habit)
}

func (app *Server) handleHabitCreateForm(w http.ResponseWriter, r *http.Request) {
	app.rend.RenderPartial(w, "habit-create", nil)
}

func (app *Server) handleHabitCreate(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("not implemented"))
}
func (app *Server) handleLogCreate(w http.ResponseWriter, r *http.Request) {
	habitSlug := r.PathValue("slug")
	userID := int64(1) // Hardcoded for demo

	if r.Method != http.MethodPost {
		app.log.Error("Invalid HTTP method")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.LogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.log.WithError(err).Error("Failed to decode request JSON")
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Set default timezone if not provided
	if req.Tz == "" {
		req.Tz = "UTC"
	}

	// Set default occurred_at if not provided
	if req.OccurredAt.IsZero() {
		req.OccurredAt = time.Now()
	}

	habitLog, err := app.repo.CreateHabitLog(userID, habitSlug, req)
	if err != nil {
		app.log.WithError(err).Error("Failed to create habit log")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Default header here? Need helper
	writeCreated(w, habitLog)
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
