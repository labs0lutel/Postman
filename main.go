package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/mux"
)

type Task struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Completed   bool   `json:"completed"`
}

var (
	mu     sync.Mutex
	tasks  = make(map[int]Task)
	nextID = 1
)

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/tasks", createTaskHandler).Methods("POST")
	r.HandleFunc("/tasks", listTasksHandler).Methods("GET")
	r.HandleFunc("/tasks/{id}", getTaskHandler).Methods("GET")
	r.HandleFunc("/tasks/{id}", updateTaskHandler).Methods("PUT")
	r.HandleFunc("/tasks/{id}", deleteTaskHandler).Methods("DELETE")

	fmt.Println("Server started at :8080")
	http.ListenAndServe(":8080", r)
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func readJSONBody(r *http.Request, dst interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func createTaskHandler(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Completed   *bool  `json:"completed"`
	}

	if err := readJSONBody(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"[ERROR]": "invalid JSON or unknown fields"})
		return
	}

	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"[ERROR]": "title is required"})
		return
	}

	completed := false
	if in.Completed != nil {
		completed = *in.Completed
	}

	mu.Lock()
	id := nextID
	nextID++
	task := Task{ID: id, Title: in.Title, Description: in.Description, Completed: completed}
	tasks[id] = task
	mu.Unlock()

	writeJSON(w, http.StatusCreated, task)
}

func listTasksHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("completed")
	var useFilter bool
	var filterVal bool

	if q != "" {
		b, err := strconv.ParseBool(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"[ERROR]": "invalid completed query parameter"})
			return
		}
		useFilter = true
		filterVal = b
	}

	mu.Lock()
	list := make([]Task, 0, len(tasks))
	for _, t := range tasks {
		if useFilter {
			if t.Completed == filterVal {
				list = append(list, t)
			}
		} else {
			list = append(list, t)
		}
	}
	mu.Unlock()

	writeJSON(w, http.StatusOK, list)
}

func getTaskHandler(w http.ResponseWriter, r *http.Request) {
	id, err := idFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"[ERROR]": "invalid id"})
		return
	}

	mu.Lock()
	t, ok := tasks[id]
	mu.Unlock()

	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"[ERROR]": "task not found"})
		return
	}

	writeJSON(w, http.StatusOK, t)
}

func updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	id, err := idFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"[ERROR]": "invalid id"})
		return
	}

	var in struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Completed   *bool   `json:"completed"`
	}

	if err := readJSONBody(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"[ERROR]": "invalid JSON or unknown fields"})
		return
	}

	mu.Lock()
	task, ok := tasks[id]
	if !ok {
		mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"[ERROR]": "task not found"})
		return
	}

	if in.Title != nil {
		trim := strings.TrimSpace(*in.Title)
		if trim == "" {
			mu.Unlock()
			writeJSON(w, http.StatusBadRequest, map[string]string{"[ERROR]": "title cannot be empty"})
			return
		}
		task.Title = trim
	}
	if in.Description != nil {
		task.Description = *in.Description
	}
	if in.Completed != nil {
		task.Completed = *in.Completed
	}

	tasks[id] = task
	mu.Unlock()

	writeJSON(w, http.StatusOK, task)
}

func deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	id, err := idFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"[ERROR]": "invalid id"})
		return
	}

	mu.Lock()
	_, ok := tasks[id]
	if !ok {
		mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"[ERROR]": "task not found"})
		return
	}
	delete(tasks, id)
	mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

func idFromRequest(r *http.Request) (int, error) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	if idStr == "" {
		return 0, errors.New("missing id")
	}
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}
