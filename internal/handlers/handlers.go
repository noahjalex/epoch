package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
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

	mux := http.NewServeMux()
	// Create a file server to serve files from the "static" directory
	fs := http.FileServer(http.Dir("./static"))

	// Handle requests for "/static/" by stripping the prefix and serving files
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Web routes
	mux.HandleFunc("/", server.handleHome)
	// mux.HandleFunc("GET /habits/{id:[0-9]+}", app.handleHabitView)

	// API routes
	// mux.HandleFunc("GET /api/habits", app.handleHabitsView)
	// mux.HandleFunc("POST /api/habits/{slug}/logs", app.handleLogCreate)
	// mux.HandleFunc("GET /api/logs", app.handleRecentLogsView)

	openServer(port)
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

	d, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(d[:500]))

	app.rend.Render(w, "home", data)
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
