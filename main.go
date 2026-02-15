package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Todo struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	mu       sync.Mutex
	todos    []Todo
	nextID   int
	filepath string
}

func NewStore(filepath string) *Store {
	s := &Store{filepath: filepath, nextID: 1}
	s.load()
	return s
}

func (s *Store) load() {
	data, err := os.ReadFile(s.filepath)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &s.todos); err != nil {
		return
	}
	for _, t := range s.todos {
		if t.ID >= s.nextID {
			s.nextID = t.ID + 1
		}
	}
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.todos, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filepath, data, 0644)
}

func (s *Store) All() []Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]Todo, len(s.todos))
	copy(result, s.todos)
	return result
}

func (s *Store) Add(title string) Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := Todo{
		ID:        s.nextID,
		Title:     title,
		Completed: false,
		CreatedAt: time.Now(),
	}
	s.nextID++
	s.todos = append(s.todos, t)
	s.save()
	return t
}

func (s *Store) Update(id int, title *string, completed *bool) (Todo, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.todos {
		if t.ID == id {
			if title != nil {
				s.todos[i].Title = *title
			}
			if completed != nil {
				s.todos[i].Completed = *completed
			}
			s.save()
			return s.todos[i], true
		}
	}
	return Todo{}, false
}

func (s *Store) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.todos {
		if t.ID == id {
			s.todos = append(s.todos[:i], s.todos[i+1:]...)
			s.save()
			return true
		}
	}
	return false
}

func main() {
	store := NewStore("todos.json")

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "static/index.html")
	})

	http.HandleFunc("/api/todos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(store.All())
		case http.MethodPost:
			var body struct {
				Title string `json:"title"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Title) == "" {
				http.Error(w, `{"error":"title is required"}`, http.StatusBadRequest)
				return
			}
			t := store.Add(strings.TrimSpace(body.Title))
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(t)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/api/todos/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		idStr := strings.TrimPrefix(r.URL.Path, "/api/todos/")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodPut:
			var body struct {
				Title     *string `json:"title"`
				Completed *bool   `json:"completed"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
				return
			}
			t, ok := store.Update(id, body.Title, body.Completed)
			if !ok {
				http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(t)
		case http.MethodDelete:
			if !store.Delete(id) {
				http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	log.Println("Server started on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
