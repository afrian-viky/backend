package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"a21hc3NpZ25tZW50/service"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

var fileService = &service.FileService{}
var aiService = &service.AIService{Client: &http.Client{}}
var store = sessions.NewCookieStore([]byte("my-key"))

func getSession(r *http.Request) *sessions.Session {
	session, _ := store.Get(r, "chat-session")
	return session
}

func main() {
	// Coba load .env kalau ada, tapi kalau nggak ada, lanjut pakai Railway env
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ No .env file found, using system environment variables")
	}

	token := os.Getenv("HUGGINGFACE_TOKEN")
	if token == "" {
		log.Fatal("❌ HUGGINGFACE_TOKEN is not set (check Railway Variables)")
	}

	newtoken := os.Getenv("COHORE_TOKEN")
	if newtoken == "" {
		log.Fatal("❌ COHORE_TOKEN is not set (check Railway Variables)")
	}

	router := mux.NewRouter()

	router.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") == "" || !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
			http.Error(w, "Invalid content type. Expected multipart/form-data", http.StatusBadRequest)
			return
		}

		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			http.Error(w, "Error parsing form data: "+err.Error(), http.StatusBadRequest)
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Error retrieving the file: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		fileContent, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "Error reading the file: "+err.Error(), http.StatusInternalServerError)
			return
		}

		table, err := fileService.ProcessFile(string(fileContent))
		if err != nil {
			http.Error(w, "Failed to process file: "+err.Error(), http.StatusBadRequest)
			return
		}

		fileQuery := r.FormValue("fileQuery")
		if fileQuery == "" {
			http.Error(w, "Question about the file is required", http.StatusBadRequest)
			return
		}

		answer, err := aiService.AnalyzeData(table, fileQuery, token)
		if err != nil {
			http.Error(w, "Error analyzing data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := map[string]string{
			"status": "success",
			"answer": answer,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}).Methods("POST", "OPTIONS")

	router.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "Invalid content type", http.StatusBadRequest)
			return
		}

		var data map[string]string
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, "Failed to parse request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		query, ok := data["query"]
		if !ok || query == "" {
			http.Error(w, "Query is required", http.StatusBadRequest)
			return
		}

		answer, err := aiService.ChatWithAI(query, newtoken)
		if err != nil {
			http.Error(w, "Failed to analyze data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := map[string]string{
			"status": "success",
			"answer": answer,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}).Methods("POST", "OPTIONS")

	// PERBAIKAN UTAMA: Enable CORS dengan konfigurasi yang benar
	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:3000",           // Development
			"https://smartdata-ai.vercel.app", // Production Vercel
			"https://*.vercel.app",            // Semua subdomain Vercel
		},
		AllowedMethods: []string{
			"GET", 
			"POST", 
			"PUT", 
			"DELETE", 
			"OPTIONS",
		},
		AllowedHeaders: []string{
			"Content-Type", 
			"Authorization",
			"Accept",
			"Origin",
			"X-Requested-With",
		},
		AllowCredentials: true, // Penting untuk cookies/sessions
		Debug: true, // Enable debugging untuk development
	}).Handler(router)

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("🚀 Server running on port %s", port)
	log.Printf("🌐 CORS enabled for: localhost:3000, smartdata-ai.vercel.app")
	log.Fatal(http.ListenAndServe(":"+port, corsHandler))
}