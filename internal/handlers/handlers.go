package handlers

import (
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

	// API routes
	// mux.HandleFunc("GET /api/habits", app.handleHabitsView)
	// mux.HandleFunc("POST /api/habits/{slug}/logs", app.handleLogCreate)
	// mux.HandleFunc("GET /api/logs", app.handleRecentLogsView)

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
